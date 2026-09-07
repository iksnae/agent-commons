# SPDX-License-Identifier: MPL-2.0
"""Check a bundle with the installed Hermes parser, without starting its CLI."""

import json
from pathlib import Path
import sys


def check_package(runtime, package, data):
    # This is version-bound test evidence, not a product dependency on Hermes internals.
    sys.path.insert(0, str(runtime))
    from hermes_cli import __version__
    from hermes_cli.agent_plugins import load_agent_plugin

    loaded = load_agent_plugin(package, data)
    assert not loaded.diagnostics, loaded.diagnostics
    assert loaded.name == "agent-commons"
    assert [skill.name for skill in loaded.skills] == ["agent-commons"]
    skill = loaded.skills[0]
    assert skill.skill_md == package / "skills/agent-commons/SKILL.md"
    for name in ("connection", "participation", "continuity"):
        assert (skill.root / "references" / (name + ".md")).is_file()
    assert (skill.root / "LICENSE").is_file()
    assert set(loaded.mcp_servers) == {"agent-commons"}
    server = loaded.mcp_servers["agent-commons"]
    assert server == {
        "command": "agent-commons",
        "args": ["connect-mcp"],
        "cwd": str(package),
        "env": {"PLUGIN_ROOT": str(package), "PLUGIN_DATA": str(data)},
    }, "portable MCP translation differs"
    native = json.loads((package / ".mcp.json").read_text())
    assert native["mcpServers"]["agent-commons"] == {
        "command": server["command"], "args": server["args"]
    }, "native and portable connection commands drifted"
    print("Hermes", __version__, "loaded the shared skill and scoped MCP configuration")


if __name__ == "__main__":
    check_package(*(Path(value).resolve() for value in sys.argv[1:]))
