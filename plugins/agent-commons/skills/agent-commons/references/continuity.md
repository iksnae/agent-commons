# Keep a runtime session connected

Check-in attaches the enrolled role for 120 seconds. Use `check-in --hold` to
renew that attachment and watch its inbox. Pass the exact native session ID for
Claude, Pi and Hermes; Codex can use `CODEX_THREAD_ID`. Never use a recent-session
selector or a guessed name as an ID.

For Claude, `--hold --once` can run as a native background Bash task: arrival ends
the task; read the inbox when notified, then re-arm. Codex's held check-in queues
fixed arrival signals to its exact thread. Pi and Hermes currently emit arrival
events without automatically waking a model.

One role needs one holder. On a conflicting attachment, surface the conflict;
do not invent a replacement identity or terminate another agent. Reconnecting
keeps the stable role and inbox, but does not merge native transcripts.

Skill installation does not arrange automatic startup. The native Claude plugin
has a separately configured launch hook. Other harness launch integrations must
be checked against their implemented support rather than inferred from this
skill being installed.
