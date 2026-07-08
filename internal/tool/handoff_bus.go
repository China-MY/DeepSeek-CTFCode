package tool

import "sync"

// HandoffBus carries handoff and completion signals from the handoff_to_agent
// and task_complete tools to the Orchestrator. It is the communication bridge
// between the tool execution layer and the orchestration layer.
//
// Handoff requests are one-shot: ConsumeHandoff / ConsumeComplete read and
// clear the pending flag so only one turn processes each request.
type HandoffBus struct {
	mu        sync.Mutex
	targetID  string
	task      string
	requested bool
	completed bool
}

// NewHandoffBus returns an initialized handoff bus.
func NewHandoffBus() *HandoffBus {
	return &HandoffBus{}
}

// RequestHandoff stores a handoff request. Called by the handoff_to_agent tool.
func (b *HandoffBus) RequestHandoff(targetID, task string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.targetID = targetID
	b.task = task
	b.requested = true
}

// ConsumeHandoff reads and clears a pending handoff request.
// ok is false when no handoff is pending.
func (b *HandoffBus) ConsumeHandoff() (targetID, task string, ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.requested {
		return "", "", false
	}
	targetID, task = b.targetID, b.task
	b.requested = false
	b.targetID = ""
	b.task = ""
	return targetID, task, true
}

// MarkComplete marks the task as complete. Called by the task_complete tool.
func (b *HandoffBus) MarkComplete() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.completed = true
}

// ConsumeComplete reads and clears the completion flag.
func (b *HandoffBus) ConsumeComplete() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.completed {
		return false
	}
	b.completed = false
	return true
}

// Reset clears both handoff and completion flags.
func (b *HandoffBus) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.requested = false
	b.completed = false
	b.targetID = ""
	b.task = ""
}
