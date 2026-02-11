package process

// Event types that match the Windows cowork-svc protocol.

// StdoutEvent is emitted when a process writes to stdout.
// The client expects "id" (not "processId") per the Cowork protocol.
type StdoutEvent struct {
	Type      string `json:"type"`
	ProcessID string `json:"id"`
	Data      string `json:"data"`
}

// StderrEvent is emitted when a process writes to stderr.
type StderrEvent struct {
	Type      string `json:"type"`
	ProcessID string `json:"id"`
	Data      string `json:"data"`
}

// ExitEvent is emitted when a process exits.
type ExitEvent struct {
	Type      string `json:"type"`
	ProcessID string `json:"id"`
	Code      int    `json:"code"`
}

// APIReachableEvent is emitted when the API becomes reachable from inside the VM.
type APIReachableEvent struct {
	Type      string `json:"type"`
	Reachable bool   `json:"reachable"`
}

// NewStdoutEvent creates a stdout event.
func NewStdoutEvent(processID, data string) StdoutEvent {
	return StdoutEvent{Type: "stdout", ProcessID: processID, Data: data}
}

// NewStderrEvent creates a stderr event.
func NewStderrEvent(processID, data string) StderrEvent {
	return StderrEvent{Type: "stderr", ProcessID: processID, Data: data}
}

// NewExitEvent creates an exit event.
func NewExitEvent(processID string, code int) ExitEvent {
	return ExitEvent{Type: "exit", ProcessID: processID, Code: code}
}

// NewAPIReachableEvent creates an API reachability event.
func NewAPIReachableEvent(reachable bool) APIReachableEvent {
	return APIReachableEvent{Type: "apiReachable", Reachable: reachable}
}
