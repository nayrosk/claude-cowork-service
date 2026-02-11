package process

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/patrickjaja/claude-cowork-service/vm"
)

// Tracker tracks spawned processes inside the VM via sdk-daemon.
type Tracker struct {
	vsock     *vm.VsockListener
	processes map[string]*ProcessInfo
	mu        sync.RWMutex
}

// ProcessInfo holds metadata about a spawned process.
type ProcessInfo struct {
	ID      string
	Cmd     string
	Args    []string
	Running bool
}

// NewTracker creates a new process tracker.
func NewTracker(vsock *vm.VsockListener) *Tracker {
	return &Tracker{
		vsock:     vsock,
		processes: make(map[string]*ProcessInfo),
	}
}

// Spawn requests the sdk-daemon to start a new process.
func (t *Tracker) Spawn(cmd string, args []string, env map[string]string, cwd string) (string, error) {
	if t.vsock == nil || !t.vsock.IsConnected() {
		return "", fmt.Errorf("sdk-daemon not connected")
	}

	resp, err := t.vsock.SendCommand(map[string]interface{}{
		"method": "spawn",
		"cmd":    cmd,
		"args":   args,
		"env":    env,
		"cwd":    cwd,
	})
	if err != nil {
		return "", fmt.Errorf("spawn command failed: %w", err)
	}

	// Parse process ID from response
	var result struct {
		ProcessID string `json:"processId"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("parsing spawn response: %w", err)
	}

	t.mu.Lock()
	t.processes[result.ProcessID] = &ProcessInfo{
		ID:      result.ProcessID,
		Cmd:     cmd,
		Args:    args,
		Running: true,
	}
	t.mu.Unlock()

	return result.ProcessID, nil
}

// Kill terminates a process by ID.
func (t *Tracker) Kill(processID string) error {
	if t.vsock == nil || !t.vsock.IsConnected() {
		return fmt.Errorf("sdk-daemon not connected")
	}

	_, err := t.vsock.SendCommand(map[string]interface{}{
		"method":    "kill",
		"processId": processID,
	})
	if err != nil {
		return err
	}

	t.mu.Lock()
	if p, ok := t.processes[processID]; ok {
		p.Running = false
	}
	t.mu.Unlock()

	return nil
}

// IsRunning checks if a process is still running.
func (t *Tracker) IsRunning(processID string) bool {
	t.mu.RLock()
	p, ok := t.processes[processID]
	t.mu.RUnlock()

	if !ok {
		return false
	}
	return p.Running
}

// MarkExited marks a process as no longer running.
func (t *Tracker) MarkExited(processID string) {
	t.mu.Lock()
	if p, ok := t.processes[processID]; ok {
		p.Running = false
	}
	t.mu.Unlock()
}
