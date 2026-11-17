# Agent Harness

A from-scratch agent harness 


# Install dependencies 

go get -u 'github.com/openai/openai-go/v3@v3.56.0'


# Setup

## Bash tool permissions

The `bash` tool runs commands via `sh -c` and supports chaining with
`&&`, `||`, `;` and `|`. Every command in a chain is validated:

- `BASH_DENYLIST` — comma-separated commands that are always rejected.
  Defaults to `rm,sudo,dd,mkfs` when unset. Deny wins on conflict: a command
  in both lists is rejected.
- `BASH_ALLOWLIST` — comma-separated allowed commands. Falls back to a
  built-in default list when unset.
- `ALLOW_ALL_COMMANDS` — set to `true` to bypass the allowlist entirely;
  the denylist still applies.

# Running 
