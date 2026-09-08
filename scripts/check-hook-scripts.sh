#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
# Execute the shipped plugin shell scripts against stand-in binaries in a
# `bundle install` layout. No Claude, no Pi, no network, no models, and no
# writes outside the scratch directory: the stand-ins only record their argv.
set -euo pipefail
cd "$(dirname "$0")/.."

hook=plugins/agent-commons/scripts/claude-session-start.sh
checkin=plugins/agent-commons/scripts/check-in.sh
connect=plugins/agent-commons/scripts/connect-mcp.sh
sh -n "$hook"
sh -n "$checkin"
sh -n "$connect"

check_dir=$(mktemp -d)
trap 'rm -rf "$check_dir"' EXIT

# `bundle install` writes the binary to the destination root and the plugin to
# <root>/plugins/agent-commons, so CLAUDE_PLUGIN_ROOT/../../agent-commons is the
# installed binary. That install deliberately adds nothing to PATH.
root="$check_dir/install"
plugin="$root/plugins/agent-commons"
mkdir -p "$plugin/scripts"
cp "$hook" "$checkin" "$connect" "$plugin/scripts/"
chmod 755 "$plugin/scripts"/*.sh

argv="$check_dir/argv"
invoked="$check_dir/invoked"

standin() {
  cat > "$1" <<'STANDIN'
#!/bin/sh
set -eu
: > "$HOOK_CHECK_ARGV"
for arg in "$@"; do printf '%s\n' "$arg" >> "$HOOK_CHECK_ARGV"; done
printf '%s\n' "$0" > "$HOOK_CHECK_INVOKED"
echo '{"hookSpecificOutput":{"hookEventName":"SessionStart"}}'
STANDIN
  chmod 755 "$1"
}
standin "$root/agent-commons"

# A stand-in that records argv but prints nothing. connect-mcp speaks JSON-RPC
# over stdio, so with a silent binary any byte on stdout is the wrapper's own.
silent_standin() {
  cat > "$1" <<'STANDIN'
#!/bin/sh
set -eu
: > "$HOOK_CHECK_ARGV"
for arg in "$@"; do printf '%s\n' "$arg" >> "$HOOK_CHECK_ARGV"; done
printf '%s\n' "$0" > "$HOOK_CHECK_INVOKED"
STANDIN
  chmod 755 "$1"
}

# A stand-in `claude` only; the hook reads its --version. Nothing named
# agent-commons is reachable on this PATH.
bare_path="$check_dir/path"
mkdir "$bare_path"
printf '#!/bin/sh\necho "2.1.236 (Claude Code)"\n' > "$bare_path/claude"
chmod 755 "$bare_path/claude"
# A second PATH that does carry the binary, for the source-checkout tier.
full_path="$check_dir/path-with-binary"
mkdir "$full_path"
cp "$bare_path/claude" "$full_path/claude"
standin "$full_path/agent-commons"

# A plugin copy with no binary above it, for the tiers that must not find one.
orphan="$check_dir/orphan/plugins/agent-commons"
mkdir -p "$orphan/scripts"
cp "$hook" "$checkin" "$connect" "$orphan/scripts/"
test ! -e "$check_dir/orphan/agent-commons"

fail() { echo "check-hook-scripts.sh: $1" >&2; exit 1; }
argv_line() { sed -n "$1p" "$argv"; }
# The recorded $0 may hold an unnormalized ../.. path; compare real locations.
invoked_path() {
  recorded=$(cat "$invoked")
  echo "$(cd "$(dirname "$recorded")" && pwd)/$(basename "$recorded")"
}

# Run one script with an empty environment plus exactly the NAME=VALUE variables
# that follow it; any remaining arguments are passed to the script itself.
run() {
  script="$1"; shift
  vars=()
  while [[ $# -gt 0 && $1 == *=* ]]; do vars+=("$1"); shift; done
  rm -f "$argv" "$invoked"
  status=0
  env -i HOME="$check_dir/home" HOOK_CHECK_ARGV="$argv" HOOK_CHECK_INVOKED="$invoked" \
    "${vars[@]}" sh "$script" "$@" < /dev/null \
    > "$check_dir/out.log" 2> "$check_dir/err.log" || status=$?
}

# 1. Tier 2, the case that has never worked: a bundle-install destination, an
#    empty environment apart from CLAUDE_PLUGIN_ROOT, and nothing on PATH. The
#    hook must find the installed binary and invoke launch-context with no
#    --config, leaving the role to the project manifest.
run "$plugin/scripts/claude-session-start.sh" PATH="$bare_path:/usr/bin:/bin" \
  CLAUDE_PLUGIN_ROOT="$plugin"
test "$status" -eq 0 || fail "hook exited $status from a bundle-install destination"
test -f "$invoked" || fail "hook never invoked the binary installed beside the plugin"
test "$(invoked_path)" = "$root/agent-commons" ||
  fail "hook ran $(invoked_path), not $root/agent-commons"
if grep -qx -- '--config' "$argv"; then fail "hook passed --config with no connection set"; fi
diff -u - "$argv" <<'EXPECTED' || fail "unexpected argv without a connection"
launch-context
--agent-type

--claude-version
2.1.236 (Claude Code)
EXPECTED

# 2. An explicit connection still pins that role: --config carries its value.
run "$plugin/scripts/claude-session-start.sh" PATH="$bare_path:/usr/bin:/bin" \
  CLAUDE_PLUGIN_ROOT="$plugin" AGENT_COMMONS_CONNECTION=/private/role.json
diff -u - "$argv" <<'EXPECTED' || fail "unexpected argv with a connection"
launch-context
--config
/private/role.json
--agent-type

--claude-version
2.1.236 (Claude Code)
EXPECTED

# 3. Tier 1 wins: an explicit binary is used even when tier 2 would resolve.
standin "$check_dir/explicit-agent-commons"
run "$plugin/scripts/claude-session-start.sh" PATH="$full_path:/usr/bin:/bin" \
  CLAUDE_PLUGIN_ROOT="$plugin" AGENT_COMMONS_BINARY="$check_dir/explicit-agent-commons" \
  AGENT_COMMONS_CLAUDE_AGENT=builder
test "$(invoked_path)" = "$check_dir/explicit-agent-commons" ||
  fail "AGENT_COMMONS_BINARY did not win: ran $(invoked_path)"
test "$(argv_line 3)" = "builder" || fail "hook dropped AGENT_COMMONS_CLAUDE_AGENT"

# 4. Tier 3: no CLAUDE_PLUGIN_ROOT, a real PATH entry, as in a source checkout.
run "$orphan/scripts/claude-session-start.sh" PATH="$full_path:/usr/bin:/bin"
test -f "$invoked" || fail "hook ignored an agent-commons on PATH"
test "$(invoked_path)" = "$full_path/agent-commons" ||
  fail "hook ran $(invoked_path), not the PATH binary"

# 5. A derived path that is absent or not executable falls through to tier 3
#    instead of failing the launch.
printf 'not executable\n' > "$check_dir/orphan/agent-commons"
chmod 644 "$check_dir/orphan/agent-commons"
run "$orphan/scripts/claude-session-start.sh" PATH="$full_path:/usr/bin:/bin" \
  CLAUDE_PLUGIN_ROOT="$orphan"
test "$(invoked_path)" = "$full_path/agent-commons" ||
  fail "a non-executable derived path did not fall through to PATH"
rm -f "$check_dir/orphan/agent-commons"

# 6. No tier resolves: exit cleanly and silently. A plugin can be installed
#    without the binary, and a SessionStart hook must never break a launch.
run "$orphan/scripts/claude-session-start.sh" PATH="$bare_path:/usr/bin:/bin" \
  CLAUDE_PLUGIN_ROOT="$orphan"
test "$status" -eq 0 || fail "hook exited $status with no binary findable"
test ! -f "$invoked" || fail "hook invoked something with no binary findable"
test ! -s "$check_dir/out.log" || fail "hook wrote stdout with no binary findable"
test ! -s "$check_dir/err.log" || fail "hook wrote stderr with no binary findable"

# 7. check-in.sh shares tiers 1 and 2: tier 2 from a bundle-install destination
#    with nothing on PATH. Its tier 3 differs and is case 9.
run "$plugin/scripts/check-in.sh" PATH="$bare_path:/usr/bin:/bin" \
  CLAUDE_PLUGIN_ROOT="$plugin" AGENT_COMMONS_CONNECTION=/private/role.json claude
test "$status" -eq 0 || fail "check-in.sh exited $status from a bundle-install destination"
test "$(invoked_path)" = "$root/agent-commons" ||
  fail "check-in.sh ran $(invoked_path), not the installed binary"
diff -u - "$argv" <<'EXPECTED' || fail "unexpected check-in.sh argv"
check-in
--config
/private/role.json
--runtime
claude
EXPECTED

# 8. check-in.sh tier 1 still wins, and tier 3 still resolves from PATH.
run "$plugin/scripts/check-in.sh" PATH="$full_path:/usr/bin:/bin" \
  AGENT_COMMONS_CONNECTION=/private/role.json \
  AGENT_COMMONS_BINARY="$check_dir/explicit-agent-commons" claude
test "$(invoked_path)" = "$check_dir/explicit-agent-commons" ||
  fail "check-in.sh ignored AGENT_COMMONS_BINARY"
run "$orphan/scripts/check-in.sh" PATH="$full_path:/usr/bin:/bin" \
  AGENT_COMMONS_CONNECTION=/private/role.json claude
test "$(invoked_path)" = "$full_path/agent-commons" ||
  fail "check-in.sh ignored an agent-commons on PATH"

# 9. check-in.sh is an explicit operator command, not a launch hook: with no
#    binary findable it must report the failure rather than exit silently.
run "$orphan/scripts/check-in.sh" PATH="$bare_path:/usr/bin:/bin" \
  CLAUDE_PLUGIN_ROOT="$orphan" AGENT_COMMONS_CONNECTION=/private/role.json claude
test "$status" -ne 0 || fail "check-in.sh reported success with no binary findable"
test -s "$check_dir/err.log" || fail "check-in.sh failed without saying why"

# 10. Tier 2 outranks tier 3: with the installed binary AND an agent-commons on
#     PATH, and no AGENT_COMMONS_BINARY to short-circuit the chain, the hook must
#     run the installed one. `bundle install` adds nothing to PATH, so a PATH hit
#     here is some other copy and must not win over the plugin's own.
run "$plugin/scripts/claude-session-start.sh" PATH="$full_path:/usr/bin:/bin" \
  CLAUDE_PLUGIN_ROOT="$plugin"
test "$(invoked_path)" = "$root/agent-commons" ||
  fail "PATH outranked the installed binary: ran $(invoked_path)"

# 11. check-in.sh applies the same executability test to its derived path: a
#     non-executable file there falls through to PATH rather than failing.
printf 'not executable\n' > "$check_dir/orphan/agent-commons"
chmod 644 "$check_dir/orphan/agent-commons"
run "$orphan/scripts/check-in.sh" PATH="$full_path:/usr/bin:/bin" \
  CLAUDE_PLUGIN_ROOT="$orphan" AGENT_COMMONS_CONNECTION=/private/role.json claude
test "$status" -eq 0 ||
  fail "check-in.sh exited $status on a non-executable derived path"
test "$(invoked_path)" = "$full_path/agent-commons" ||
  fail "check-in.sh ran $(invoked_path) from a non-executable derived path"
rm -f "$check_dir/orphan/agent-commons"

# 12. Silence is for "no project here", not for "the operator configured
#     something that is broken". With AGENT_COMMONS_CONNECTION set and no
#     binary findable, the intent is unambiguous and the hook must say so.
run "$orphan/scripts/claude-session-start.sh" PATH="$bare_path:/usr/bin:/bin" \
  CLAUDE_PLUGIN_ROOT="$orphan" AGENT_COMMONS_CONNECTION=/private/role.json
test "$status" -ne 0 || fail "a configured connection with no binary exited 0"
test "$status" -ne 2 || fail "the hook used exit 2, which blocks the launch"
test ! -f "$invoked" || fail "hook invoked something with no binary findable"
grep -q 'AGENT_COMMONS_CONNECTION' "$check_dir/err.log" ||
  fail "the hook did not say why it could not check in: $(cat "$check_dir/err.log")"
test ! -s "$check_dir/out.log" || fail "the hook wrote diagnostics to stdout"

# 13. connect-mcp.sh is the plugin tarball's MCP command: the tarball can never
#     know its destination, so the wrapper resolves the binary at run time. Tier
#     2 from a bundle-install destination with nothing on PATH, and the
#     subcommand it must always supply.
run "$plugin/scripts/connect-mcp.sh" PATH="$bare_path:/usr/bin:/bin" \
  CLAUDE_PLUGIN_ROOT="$plugin"
test "$status" -eq 0 || fail "connect-mcp.sh exited $status from a bundle-install destination"
test "$(invoked_path)" = "$root/agent-commons" ||
  fail "connect-mcp.sh ran $(invoked_path), not the installed binary"
diff -u - "$argv" <<'EXPECTED' || fail "unexpected connect-mcp.sh argv"
connect-mcp
EXPECTED

# 14. The MCP client may pass its own flags; the wrapper forwards them after the
#     subcommand rather than swallowing them.
run "$plugin/scripts/connect-mcp.sh" PATH="$bare_path:/usr/bin:/bin" \
  CLAUDE_PLUGIN_ROOT="$plugin" --config /private/role.json
diff -u - "$argv" <<'EXPECTED' || fail "connect-mcp.sh dropped forwarded arguments"
connect-mcp
--config
/private/role.json
EXPECTED

# 15. connect-mcp.sh tier 1 wins over a resolvable tier 2 and a PATH binary.
run "$plugin/scripts/connect-mcp.sh" PATH="$full_path:/usr/bin:/bin" \
  CLAUDE_PLUGIN_ROOT="$plugin" AGENT_COMMONS_BINARY="$check_dir/explicit-agent-commons"
test "$(invoked_path)" = "$check_dir/explicit-agent-commons" ||
  fail "connect-mcp.sh ignored AGENT_COMMONS_BINARY: ran $(invoked_path)"

# 16. Tier 2 outranks tier 3 for connect-mcp.sh too: an unrelated copy on PATH
#     must not win over the binary installed beside the plugin.
run "$plugin/scripts/connect-mcp.sh" PATH="$full_path:/usr/bin:/bin" \
  CLAUDE_PLUGIN_ROOT="$plugin"
test "$(invoked_path)" = "$root/agent-commons" ||
  fail "connect-mcp.sh let PATH outrank the installed binary: ran $(invoked_path)"

# 17. Tier 3: no CLAUDE_PLUGIN_ROOT, as in a source checkout that keeps the
#     binary on PATH.
run "$orphan/scripts/connect-mcp.sh" PATH="$full_path:/usr/bin:/bin"
test "$(invoked_path)" = "$full_path/agent-commons" ||
  fail "connect-mcp.sh ignored an agent-commons on PATH"

# 18. A derived path that exists but is not executable falls through to PATH.
printf 'not executable\n' > "$check_dir/orphan/agent-commons"
chmod 644 "$check_dir/orphan/agent-commons"
run "$orphan/scripts/connect-mcp.sh" PATH="$full_path:/usr/bin:/bin" \
  CLAUDE_PLUGIN_ROOT="$orphan"
test "$status" -eq 0 ||
  fail "connect-mcp.sh exited $status on a non-executable derived path"
test "$(invoked_path)" = "$full_path/agent-commons" ||
  fail "connect-mcp.sh ran $(invoked_path) from a non-executable derived path"
rm -f "$check_dir/orphan/agent-commons"

# 19. No tier resolves. Starting an MCP server is an explicit request, not a
#     launch hook, so the failure must be reported rather than swallowed.
run "$orphan/scripts/connect-mcp.sh" PATH="$bare_path:/usr/bin:/bin" \
  CLAUDE_PLUGIN_ROOT="$orphan"
test "$status" -ne 0 || fail "connect-mcp.sh reported success with no binary findable"
test ! -f "$invoked" || fail "connect-mcp.sh invoked something with no binary findable"
test -s "$check_dir/err.log" || fail "connect-mcp.sh failed without saying why"

# 20. connect-mcp speaks JSON-RPC over stdio, so the wrapper must contribute
#     nothing to stdout on any tier where it execs. Its diagnostics belong on
#     stderr, which is what makes case 19's loud failure safe to prefer over the
#     session hook's silent exit. A single echo added later would corrupt every
#     MCP session and pass every case above, so pin the silence on all three
#     tiers against stand-ins that print nothing of their own.
silent_standin "$root/agent-commons"
silent_standin "$full_path/agent-commons"
silent_standin "$check_dir/silent-agent-commons"
for tier in 1 2 3; do
  case "$tier" in
    1) run "$plugin/scripts/connect-mcp.sh" PATH="$bare_path:/usr/bin:/bin" \
      CLAUDE_PLUGIN_ROOT="$plugin" AGENT_COMMONS_BINARY="$check_dir/silent-agent-commons" ;;
    2) run "$plugin/scripts/connect-mcp.sh" PATH="$bare_path:/usr/bin:/bin" \
      CLAUDE_PLUGIN_ROOT="$plugin" ;;
    3) run "$orphan/scripts/connect-mcp.sh" PATH="$full_path:/usr/bin:/bin" ;;
  esac
  test "$status" -eq 0 || fail "connect-mcp.sh exited $status on tier $tier"
  test -f "$invoked" || fail "connect-mcp.sh never reached a binary on tier $tier"
  test ! -s "$check_dir/out.log" ||
    fail "connect-mcp.sh wrote to the JSON-RPC stream on tier $tier: $(cat "$check_dir/out.log")"
  test ! -s "$check_dir/err.log" ||
    fail "connect-mcp.sh wrote stderr on a successful tier $tier: $(cat "$check_dir/err.log")"
done

echo "Plugin scripts verified: installed binary, pinned connection, explicit binary, PATH binary, non-executable fallthrough, absent binary, tier order, configured-but-missing binary, check-in.sh chain, connect-mcp.sh chain, connect-mcp.sh stdio silence."
