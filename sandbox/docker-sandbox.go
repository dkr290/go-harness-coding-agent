// Package sandbox owns the Docker container that executes agent shell
// commands, and manages the container's lifecycle.
//
// The bash tool asks the sandbox to run things; the sandbox knows Docker; the
// bash tool stays Docker-unaware.
package sandbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dkr290/go-harness-coding-agent/config"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	mobyclient "github.com/moby/moby/client"
)

// Executor runs a shell command inside the sandbox and reports the outcome:
// the process exit code plus the captured stdout and stderr.
type Executor interface {
	Execute(ctx context.Context, command string, timeoutSeconds int) (exitCode int, stdout string, stderr string, err error)
}

// Sandbox owns a Docker container for the agent's session.
//
// Lifecycle:
//   - Start() — clean up orphans from prior sessions, create and start a fresh
//     container, bind-mount the workspace.
//   - Execute(command) — run a shell command inside the container, return the
//     combined output plus exit code.
//   - Stop() — stop and remove the container.
type Sandbox struct {
	client                 *mobyclient.Client
	containerName          string
	containerID            string
	hostWorkspacePath      string
	containerWorkspacePath string
}

// New connects to the Docker daemon and returns a Sandbox ready to be started.
// The hostWorkspacePath is the directory bind-mounted into the container (the
// agent's workspace); the in-container path defaults to config.SandboxWorkspacePath.
func New(hostWorkspacePath string) (*Sandbox, error) {
	if hostWorkspacePath == "" {
		hostWorkspacePath = config.SandboxWorkspacePath
	}
	cli, err := mobyclient.New(mobyclient.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("sandbox: cannot create docker client: %w", err)
	}
	return &Sandbox{
		client:                 cli,
		containerName:          config.SandboxContainerPrefix + newContainerSuffix(),
		hostWorkspacePath:      hostWorkspacePath,
		containerWorkspacePath: config.SandboxWorkspacePath,
	}, nil
}

// Start cleans up any orphan containers from prior sessions, then creates and
// starts a fresh container with the workspace bind-mounted.
func (s *Sandbox) Start() error {
	ctx := context.Background()
	if err := s.cleanupOrphans(ctx); err != nil {
		return err
	}
	if err := s.createAndStart(ctx); err != nil {
		return err
	}
	return s.waitForRunning(ctx)
}

// createAndStart creates and starts a new container with the workspace
// bind-mounted.
func (s *Sandbox) createAndStart(ctx context.Context) error {
	result, err := s.client.ContainerCreate(ctx, mobyclient.ContainerCreateOptions{
		Config: &container.Config{
			Image:      config.SandboxImage,
			Cmd:        []string{"sleep", "infinity"},
			Tty:        true,
			WorkingDir: s.containerWorkspacePath,
		},
		HostConfig: &container.HostConfig{
			Binds: []string{fmt.Sprintf("%s:%s", s.hostWorkspacePath, s.containerWorkspacePath)},
		},
		Name: s.containerName,
	})
	if err != nil {
		return fmt.Errorf("sandbox: failed to create container %q: %w", s.containerName, err)
	}
	s.containerID = result.ID
	if _, err := s.client.ContainerStart(ctx, result.ID, mobyclient.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("sandbox: failed to start container %q: %w", s.containerName, err)
	}
	return nil
}

// Execute runs a shell command inside the running container.
//
// The command is wrapped in `timeout N bash -c "..."` so the container's own
// `timeout` utility handles it — that way the container itself kills a hung
// process, not the caller.
//
// Returns (exitCode, stdout, stderr). Times out if the command runs longer
// than timeoutSeconds.
func (s *Sandbox) Execute(ctx context.Context, command string, timeoutSeconds int) (int, string, string, error) {
	if s.containerID == "" {
		return -1, "", "", fmt.Errorf("sandbox: execute called before start")
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = config.DefaultBashTimeout
	}

	exec, err := s.client.ExecCreate(ctx, s.containerID, mobyclient.ExecCreateOptions{
		Cmd:          []string{"timeout", strconv.Itoa(timeoutSeconds), "bash", "-c", shellQuote(command)},
		WorkingDir:   s.containerWorkspacePath,
		AttachStdout: true,
		AttachStderr: true,
		TTY:          false,
	})
	if err != nil {
		return -1, "", "", fmt.Errorf("sandbox: failed to create exec: %w", err)
	}

	attach, err := s.client.ExecAttach(ctx, exec.ID, mobyclient.ExecAttachOptions{TTY: false})
	if err != nil {
		return -1, "", "", fmt.Errorf("sandbox: failed to attach exec: %w", err)
	}
	hijacked := attach.HijackedResponse
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	if _, err := stdcopy.StdCopy(stdout, stderr, hijacked.Reader); err != nil {
		hijacked.Close()
		return -1, "", "", fmt.Errorf("sandbox: failed to stream exec output: %w", err)
	}
	hijacked.Close()

	execInfo, err := s.client.ExecInspect(ctx, exec.ID, mobyclient.ExecInspectOptions{})
	if err != nil {
		return -1, "", "", fmt.Errorf("sandbox: failed to inspect exec: %w", err)
	}

	return execInfo.ExitCode, stdout.String(), stderr.String(), nil
}

// Stop stops and removes the container. It is a no-op if the sandbox never
// started.
func (s *Sandbox) Stop() error {
	if s.containerID == "" {
		return nil
	}
	ctx := context.Background()
	stopTimeout := 5
	if _, err := s.client.ContainerStop(ctx, s.containerID, mobyclient.ContainerStopOptions{Timeout: &stopTimeout}); err != nil {
		return fmt.Errorf("sandbox: failed to stop container %q: %w", s.containerName, err)
	}
	if _, err := s.client.ContainerRemove(ctx, s.containerID, mobyclient.ContainerRemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("sandbox: failed to remove container %q: %w", s.containerName, err)
	}
	s.containerID = ""
	return nil
}

// cleanupOrphans removes any leftover containers from prior sessions. Containers
// whose name starts with the configured prefix are ours (or were ours) and
// should be removed.
func (s *Sandbox) cleanupOrphans(ctx context.Context) error {
	list, err := s.client.ContainerList(ctx, mobyclient.ContainerListOptions{All: true})
	if err != nil {
		return fmt.Errorf("sandbox: failed to list containers: %w", err)
	}
	for _, c := range list.Items {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		if !strings.HasPrefix(name, config.SandboxContainerPrefix) {
			continue
		}
		if _, err := s.client.ContainerRemove(ctx, c.ID, mobyclient.ContainerRemoveOptions{Force: true}); err != nil {
			return fmt.Errorf("sandbox: failed to remove orphan container %q: %w", name, err)
		}
	}
	return nil
}

// waitForRunning blocks until the container's state is "running", up to the
// configured startup timeout.
func (s *Sandbox) waitForRunning(ctx context.Context) error {
	deadline := time.Now().Add(time.Duration(config.SandboxStartupTimeout) * time.Second)
	for {
		result, err := s.client.ContainerInspect(ctx, s.containerID, mobyclient.ContainerInspectOptions{})
		if err != nil {
			return fmt.Errorf("sandbox: failed to inspect container %q: %w", s.containerName, err)
		}
		if result.Container.State != nil && result.Container.State.Status == container.StateRunning {
			return nil
		}
		if !time.Now().Before(deadline) {
			status := "unknown"
			if result.Container.State != nil {
				status = string(result.Container.State.Status)
			}
			return fmt.Errorf("sandbox: container %q did not reach 'running' state within %d seconds (current: %s)",
				s.containerName, config.SandboxStartupTimeout, status)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// newContainerSuffix returns an 8-char hex suffix for the container name.
func newContainerSuffix() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:8]
}

// shellQuote quotes a string for safe interpolation into a shell command.
//
// We use single-quotes and escape any single-quote in the string itself by
// ending the quote, adding an escaped ', and reopening the quote.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

var (
	currentMu   sync.RWMutex
	currentExec Executor
)

// Set installs the Executor used by the bash tool to run commands. It is the
// module-level sandbox setter: the bash tool asks for the current executor
// (normally a *Sandbox) and runs commands through it.
func Set(s Executor) {
	currentMu.Lock()
	defer currentMu.Unlock()
	currentExec = s
}

// Get returns the currently installed Executor, or nil if none is set.
func Get() Executor {
	currentMu.RLock()
	defer currentMu.RUnlock()
	return currentExec
}
