package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"ctfcode/internal/tool"
)

// handoffToAgent lets the current agent hand a task to another agent.
// The Orchestrator intercepts this and switches the active agent.
type handoffToAgent struct {
	bus *tool.HandoffBus // shared with the Orchestrator
}

// RegisterHandoffTools registers the handoff_to_agent and task_complete tools
// with the given HandoffBus. Must be called during boot when multi-agent mode
// is enabled. When called with a nil bus, no-op tools are registered (they
// will be available but won't cause handoff — useful for single-agent mode).
func RegisterHandoffTools(bus *tool.HandoffBus) {
	tool.RegisterBuiltin(handoffToAgent{bus: bus})
	tool.RegisterBuiltin(taskComplete{bus: bus})
}

func (h handoffToAgent) Name() string { return "handoff_to_agent" }

func (h handoffToAgent) Description() string {
	return "Hand off the current task to another agent in the multi-agent system. " +
		"Use this when the task requires capabilities or tools that belong to another agent role " +
		"(for example, a planner handing implementation work to a builder, or a researcher handing " +
		"findings back to the planner). The target agent receives a handoff message with the task " +
		"description and takes over execution. All agents share the conversation history so context " +
		"is preserved across handoffs."
}

func (h handoffToAgent) Schema() json.RawMessage {
	return json.RawMessage(`{
"type":"object",
"properties":{
  "target_agent_id":{
    "type":"string",
    "description":"The ID of the agent to hand off to (e.g. 'planner', 'builder', 'operator', 'research')."
  },
  "task":{
    "type":"string",
    "description":"The task or instructions for the target agent. Include relevant context, findings, and what the target should do next."
  }
},
"required":["target_agent_id","task"]
}`)
}

func (h handoffToAgent) ReadOnly() bool { return false }

func (h handoffToAgent) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		TargetAgentID string `json:"target_agent_id"`
		Task          string `json:"task"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("handoff_to_agent: invalid args: %w", err)
	}
	if p.TargetAgentID == "" {
		return "", fmt.Errorf("handoff_to_agent: target_agent_id is required")
	}
	if p.Task == "" {
		return "", fmt.Errorf("handoff_to_agent: task is required")
	}

	if h.bus != nil {
		h.bus.RequestHandoff(p.TargetAgentID, p.Task)
	}

	return fmt.Sprintf("[Handoff requested to agent %q]: The orchestrator will switch to %q on the next turn. Task: %s",
		p.TargetAgentID, p.TargetAgentID, p.Task), nil
}

// taskComplete signals that the current task is finished and the session
// should return control to the user.
type taskComplete struct {
	bus *tool.HandoffBus
}

func (t taskComplete) Name() string { return "task_complete" }

func (t taskComplete) Description() string {
	return "Signal that the current task is complete and the session should return control to the user. " +
		"Use this when the agent has finished all work and the results are ready. " +
		"The orchestrator will stop the current agent loop and present the results."
}

func (t taskComplete) Schema() json.RawMessage {
	return json.RawMessage(`{
"type":"object",
"properties":{
  "result":{
    "type":"string",
    "description":"A summary of what was accomplished (optional)."
  }
}
}`)
}

func (t taskComplete) ReadOnly() bool { return false }

func (t taskComplete) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Result string `json:"result"`
	}
	_ = json.Unmarshal(args, &p)

	if t.bus != nil {
		t.bus.MarkComplete()
	}

	summary := "Task complete."
	if p.Result != "" {
		summary = "Task complete: " + p.Result
	}
	return summary, nil
}
