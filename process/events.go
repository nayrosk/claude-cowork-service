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
// Client reads a.exitCode, a.signal, a.oomKillCount.
type ExitEvent struct {
	Type         string `json:"type"`
	ProcessID    string `json:"id"`
	ExitCode     int    `json:"exitCode"`
	Signal       string `json:"signal,omitempty"`
	OOMKillCount int    `json:"oomKillCount,omitempty"`
}

// APIReachableEvent is emitted when the API becomes reachable from inside the VM.
// Client validates: { reachability: "unknown"|"reachable"|"probably_unreachable"|"unreachable", willTryRecover: bool }
type APIReachableEvent struct {
	Type           string `json:"type"`
	Reachability   string `json:"reachability"`
	WillTryRecover bool   `json:"willTryRecover"`
}

// ErrorEvent is emitted when a process-level error occurs.
// Client handles case "error" events with {id, message, fatal} fields.
type ErrorEvent struct {
	Type      string `json:"type"`
	ProcessID string `json:"id"`
	Message   string `json:"message"`
	Fatal     bool   `json:"fatal"`
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
	return ExitEvent{Type: "exit", ProcessID: processID, ExitCode: code}
}

// NewExitEventWithSignal creates an exit event for signal-caused exits.
func NewExitEventWithSignal(processID string, code int, signal string) ExitEvent {
	return ExitEvent{Type: "exit", ProcessID: processID, ExitCode: code, Signal: signal}
}

// NewAPIReachableEvent creates an API reachability event.
func NewAPIReachableEvent(reachable bool) APIReachableEvent {
	reachability := "unreachable"
	if reachable {
		reachability = "reachable"
	}
	return APIReachableEvent{Type: "apiReachability", Reachability: reachability, WillTryRecover: false}
}

// NewErrorEvent creates a process error event.
func NewErrorEvent(processID string, message string, fatal bool) ErrorEvent {
	return ErrorEvent{Type: "error", ProcessID: processID, Message: message, Fatal: fatal}
}
