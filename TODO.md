# Conteinerazied Execution with docker 

## Design Descisions

- Session scoped containers not per command
- We bind-mount the Workspace 
- Orphan cleanup on startup
- Placeholder image for now , custom dockerfile something like ubuntu image. 
- wiring pattern -module level sandbox with setter bash needs to be able to run docker container
Sandbox, setSandbox(sandbox). The bash tool needs to set this variable then this is the sandbox to use.



1. config. we need to have first 
SANDBOX_IMAGE:
SANDBOX_WORKSPACE_PATH= Default is  "/workspace"
SANDBOX_STARTUP_TIMEOUT = default  30
SANDBOX_CONTAINER_PREFIX = "agent-harness-"
