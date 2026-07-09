package agent

import (
	"context"
	"os"
	"fmt"

	"ctfcode/internal/event"
	"ctfcode/internal/provider"
	"ctfcode/internal/sandbox"
	"ctfcode/internal/tool"
)

// KnowledgeProvider supplies knowledge entries that can be mounted into
// an agent's system prompt context.
type KnowledgeProvider interface {
	Name() string
	MountBlock(agentID string, mountedNames []string) string
}

// ExperienceProvider records and recalls agent experiences for self-learning.
type ExperienceProvider interface {
	Name() string
	FormatBlock(agentID string, maxEntries int) string
	RecordExperience(agentID, task, outcome string, lessons, patterns []string)
}

// HandoffApprover is called before a handoff is executed, allowing the
// frontend to request user confirmation before switching agents.
type HandoffApprover interface {
	// RequestHandoffApproval asks the user to approve a handoff.
	// Returns true if approved, false if rejected.
	RequestHandoffApproval(ctx context.Context, from, to, task string) (bool, error)
}

// Orchestrator manages multiple Agent instances in a multi-agent session.
// It implements the Runner interface so it can be used anywhere a single Agent
// or Coordinator is used.
//
// The Orchestrator holds an AgentRegistry and manages switching between agents
// via the handoff_to_agent / task_complete tools. Each agent gets its own
// Session (with the agent-specific system prompt) and its own filtered tool
// Registry. Agents share the conversation history through injected handoff
// messages.
//
// When the registry is empty or contains only the default agent, the
// Orchestrator delegates to the default agent directly (single-agent mode).
type Orchestrator struct {
	registry    *AgentRegistry
	defaultProv provider.Provider
	defaultTools *tool.Registry
	defaultAgent *Agent

	// per-agent instances, created lazily on first use
	agents map[string]*Agent
	// agentConfigs mirrors the registry for quick lookup
	agentConfigs map[string]AgentConfig

	activeID string // current active agent ID
	sink     event.Sink

	// handoffBus is shared with handoff_to_agent and task_complete tools
	handoffBus *tool.HandoffBus
	// handoffApprover is called before executing a handoff to request user approval.
	handoffApprover HandoffApprover
	// knowledgeBase provides mounted knowledge entries for agent context.
	knowledgeBase KnowledgeProvider

	// expStore records and recalls agent experiences for self-learning.
	expStore ExperienceProvider


	// agentOptsFunc creates agent.Options for each agent (shares common settings)
	agentOptsFunc func(agentConfig AgentConfig) Options
	// agentSinkFunc creates an event.Sink for each agent
	agentSinkFunc func(agentID string) event.Sink
	// providerForAgent resolves the provider for a given agent config
	providerForAgent func(agentConfig AgentConfig) (provider.Provider, error)
}

// OrchestratorOption configures the Orchestrator.
type OrchestratorOption func(*Orchestrator)

// WithHandoffBus sets the handoff bus for the orchestrator.
func WithHandoffBus(bus *tool.HandoffBus) OrchestratorOption {
	return func(o *Orchestrator) {
		o.handoffBus = bus
	}
}

// WithHandoffApprover sets the handoff approval callback for the orchestrator.
// When set, every handoff_to_agent call will first request user approval.
func WithHandoffApprover(a HandoffApprover) OrchestratorOption {
	return func(o *Orchestrator) {
		o.handoffApprover = a
	}
}

// WithAgentProviderResolver sets the function used to resolve a provider for
// a given agent config. When not set, the default provider is used for all.
func WithAgentProviderResolver(fn func(AgentConfig) (provider.Provider, error)) OrchestratorOption {
	return func(o *Orchestrator) {
		o.providerForAgent = fn
	}
}

// WithAgentSink sets a function that returns a sink for a given agent ID.
func WithAgentSink(fn func(agentID string) event.Sink) OrchestratorOption {
	return func(o *Orchestrator) {
		o.agentSinkFunc = fn
	}
}

// WithKnowledgeBase sets the knowledge provider for mounted knowledge entries.
func WithKnowledgeBase(kb KnowledgeProvider) OrchestratorOption {
	return func(o *Orchestrator) {
		o.knowledgeBase = kb
	}
}

// WithExperienceStore sets the experience store for self-learning.
func WithExperienceStore(exp ExperienceProvider) OrchestratorOption {
	return func(o *Orchestrator) {
		o.expStore = exp
	}
}


// NewOrchestrator creates a multi-agent orchestrator wrapping the given
// registry, default provider, and default tool registry. The defaultAgent is
// used when the registry has no agents configured.
func NewOrchestrator(
	registry *AgentRegistry,
	defaultProv provider.Provider,
	defaultTools *tool.Registry,
	defaultAgent *Agent,
	optsFunc func(AgentConfig) Options,
	sink event.Sink,
	options ...OrchestratorOption,
) *Orchestrator {
	o := &Orchestrator{
		registry:      registry,
		defaultProv:   defaultProv,
		defaultTools:  defaultTools,
		defaultAgent:  defaultAgent,
		agents:        make(map[string]*Agent),
		agentConfigs:  make(map[string]AgentConfig),
		activeID:      DefaultAgentID,
		sink:          sink,
		agentOptsFunc: optsFunc,
		agentSinkFunc: func(agentID string) event.Sink { return sink },
	}

	for _, cfg := range registry.List() {
		o.agentConfigs[cfg.ID] = cfg
	}

	for _, opt := range options {
		opt(o)
	}

	return o
}

// ensureHandoffBus returns the handoff bus, creating a default one if needed.
func (o *Orchestrator) ensureHandoffBus() *tool.HandoffBus {
	if o.handoffBus == nil {
		o.handoffBus = tool.NewHandoffBus()
	}
	return o.handoffBus
}

// agentForID returns the Agent instance for the given agent ID, creating it
// lazily if needed.
func (o *Orchestrator) agentForID(ctx context.Context, id string) (*Agent, error) {
	// If this is the default agent, return it directly.
	if id == DefaultAgentID || id == "" {
		return o.defaultAgent, nil
	}

	// Return cached instance if available.
	if a, ok := o.agents[id]; ok {
		return a, nil
	}

	// Look up the agent config.
	cfg, ok := o.agentConfigs[id]
	if !ok {
		return nil, fmt.Errorf("unknown agent %q", id)
	}

	// Resolve the provider for this agent.
	prov := o.defaultProv
	if o.providerForAgent != nil {
		var err error
		prov, err = o.providerForAgent(cfg)
		if err != nil {
			return nil, fmt.Errorf("provider for agent %q: %w", id, err)
		}
	}

	// Build the system prompt with optional knowledge and experience injection.
	sysPrompt := cfg.SystemPrompt

	// If SystemPrompt is empty but SystemPromptFile is set, load from file.
	if sysPrompt == "" && cfg.SystemPromptFile != "" {
		content, err := os.ReadFile(cfg.SystemPromptFile)
		if err == nil {
			sysPrompt = string(content)
		} else {
			o.sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn,
				Text: fmt.Sprintf("failed to load system prompt file %q: %v", cfg.SystemPromptFile, err),
			})
		}
	}

	if sysPrompt == "" {
		sysPrompt = o.defaultAgent.systemPrompt()
	}

	// Mount knowledge entries for this agent.
	if o.knowledgeBase != nil && len(cfg.MountedKnowledge) > 0 {
		kb := o.knowledgeBase.MountBlock(id, cfg.MountedKnowledge)
		if kb != "" {
			sysPrompt += kb
		}
	}

	// Inject past experiences for self-learning.
	if o.expStore != nil && cfg.EnableLearning {
		exp := o.expStore.FormatBlock(id, 5)
		if exp != "" {
			sysPrompt += exp
		}
	}


	// Create filtered tool registry.
	tools := o.filterToolsForAgent(cfg)

	// Create the agent's session.
	sess := NewSession(sysPrompt)

	// Create the agent.
	opts := o.agentOptsFunc(cfg)
	a := New(prov, tools, sess, opts, o.agentSinkFunc(id))
	o.agents[id] = a

	return a, nil
}

// filterToolsForAgent creates a tool registry containing only the tools
// allowed by the agent's ACL. When AllowedTools is nil or ["*"], all tools
// from the default registry are included.
func (o *Orchestrator) filterToolsForAgent(cfg AgentConfig) *tool.Registry {
	reg := tool.NewRegistry()

	if cfg.AllowsAllTools() {
		// Include all default tools, except handoff meta-tools that the
		// orchestrator manages at its own level.
		for _, name := range o.defaultTools.Names() {
			if t, ok := o.defaultTools.Get(name); ok {
				reg.Add(t)
			}
		}
		return reg
	}

	// Include only explicitly allowed tools.
	for _, name := range cfg.AllowedTools {
		if name == handoffToAgentToolName || name == taskCompleteToolName {
			// These are registered separately below.
			continue
		}
		if name == ToolAllAllowed {
			continue
		}
		if t, ok := o.defaultTools.Get(name); ok {
			reg.Add(t)
		}
	}

	return reg
}

const (
	handoffToAgentToolName = "handoff_to_agent"
	taskCompleteToolName   = "task_complete"
)

// resolveActiveAgent determines which agent should handle a new user input.
// Starts with the first configured agent, or falls back to the default.
func (o *Orchestrator) resolveActiveAgent() string {
	if o.registry != nil && o.registry.IsMultiAgent() {
		ids := o.registry.IDs()
		if len(ids) > 0 && ids[0] != DefaultAgentID {
			return ids[0]
		}
	}
	return DefaultAgentID
}

// Run executes the multi-agent orchestration loop. It implements Runner.
func (o *Orchestrator) Run(ctx context.Context, input string) error {
	// Determine the initial agent.
	if o.activeID == "" || o.activeID == DefaultAgentID {
		o.activeID = o.resolveActiveAgent()
	}

	o.sink.Emit(event.Event{Kind: event.TurnStarted})

	// In single-agent mode, delegate directly.
	if !o.isMultiAgent() {
		return o.defaultAgent.Run(ctx, input)
	}

	o.sink.Emit(event.Event{
		Kind: event.Phase,
		Text: fmt.Sprintf("orchestrator · active agent: %s", o.activeID),
	})

	handoffBus := o.ensureHandoffBus()

	// The orchestrator processes the input and loops through agent handoffs.
	// Each agent runs in its own session with its own tool set. The handoff
	// bus bridges the gap: when handoff_to_agent or task_complete is called,
	// the Orchestrator observes it and switches.
	currentInput := input

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Emit an event showing which agent is now active.
		if cfg, ok := o.agentConfigs[o.activeID]; ok {
			o.sink.Emit(event.Event{
				Kind: event.Phase,
				Text: fmt.Sprintf("agent %q (%s) · active", cfg.ID, cfg.Name),
			})
		}

		// Get or create the agent for this ID.
		activeAgent, err := o.agentForID(ctx, o.activeID)
		if err != nil {
			return fmt.Errorf("orchestrator: %w", err)
		}

		// Run the agent with the current input.
		runErr := activeAgent.Run(ctx, currentInput)

		// After the agent finishes, check for handoff or completion requests.
		if targetID, task, ok := handoffBus.ConsumeHandoff(); ok {
			// Validate the handoff target.
			currentCfg, hasCfg := o.agentConfigs[o.activeID]
			if hasCfg && !currentCfg.MayHandoffTo(targetID) {
				// Deny the handoff: the current agent is not allowed to hand
				// off to the requested target.
				o.sink.Emit(event.Event{
					Kind:  event.Notice,
					Level: event.LevelWarn,
					Text:  fmt.Sprintf("orchestrator: agent %q cannot hand off to %q (not in allowed_handoffs)", o.activeID, targetID),
				})
			} else {
				// Request user approval before executing the handoff.
				if o.handoffApprover != nil {
					approved, err := o.handoffApprover.RequestHandoffApproval(ctx, o.activeID, targetID, task)
					if err != nil {
						return fmt.Errorf("orchestrator: handoff approval: %w", err)
					}
					if !approved {
						o.sink.Emit(event.Event{
							Kind: event.Notice,
							Text: fmt.Sprintf("handoff from %s to %s rejected by user", o.activeID, targetID),
						})
						return runErr
					}
				}

				// Inject the handoff message into the target agent's session.
				targetAgent, err := o.agentForID(ctx, targetID)
				if err != nil {
					return fmt.Errorf("orchestrator: handoff target %q: %w", targetID, err)
				}

				handoffMsg := fmt.Sprintf("[Handoff from %s]: %s", o.activeID, task)
				targetAgent.Session().Add(provider.Message{
					Role:    provider.RoleUser,
					Content: handoffMsg,
				})

				o.activeID = targetID
				currentInput = task

				o.sink.Emit(event.Event{
					Kind: event.Phase,
					Text: fmt.Sprintf("handoff: %s -> %s (%s)", o.activeID, targetID, task),
				})
				continue
			}
		}

		if handoffBus.ConsumeComplete() {
			o.sink.Emit(event.Event{Kind: event.TurnDone})
			return runErr
		}

		// No handoff or completion requested. Return the agent's result.
		return runErr
	}
}

// isMultiAgent reports whether the orchestrator has more than one agent.
func (o *Orchestrator) isMultiAgent() bool {
	return o.registry != nil && o.registry.IsMultiAgent()
}

// ActiveAgentID returns the currently active agent's ID.
func (o *Orchestrator) ActiveAgentID() string {
	return o.activeID
}

// Registry returns the underlying agent registry.
func (o *Orchestrator) Registry() *AgentRegistry {
	return o.registry
}

// ActiveAgentConfig returns the config of the currently active agent.
func (o *Orchestrator) ActiveAgentConfig() (AgentConfig, bool) {
	if cfg, ok := o.agentConfigs[o.activeID]; ok {
		return cfg, true
	}
	return AgentConfig{}, false
}

// ResetActiveAgent resets the active agent to the initial state.
func (o *Orchestrator) ResetActiveAgent() {
	o.activeID = o.resolveActiveAgent()
}

// HasAgent reports whether an agent with the given ID is registered.
func (o *Orchestrator) HasAgent(id string) bool {
	if id == DefaultAgentID {
		return true
	}
	_, ok := o.agentConfigs[id]
	return ok
}

// SetHandoffApprover sets the handoff approval callback.
func (o *Orchestrator) SetHandoffApprover(a HandoffApprover) {
	o.handoffApprover = a
}

// SetPlanMode propagates the plan mode setting to all created agents.
func (o *Orchestrator) SetPlanMode(v bool) {
	if o.defaultAgent != nil {
		o.defaultAgent.SetPlanMode(v)
	}
	for _, a := range o.agents {
		a.SetPlanMode(v)
	}
}

// SetPlanModeReadOnlyTrustGate propagates to all created agents.
func (o *Orchestrator) SetPlanModeReadOnlyTrustGate(g PlanModeReadOnlyTrustGate) {
	if o.defaultAgent != nil {
		o.defaultAgent.SetPlanModeReadOnlyTrustGate(g)
	}
	for _, a := range o.agents {
		a.SetPlanModeReadOnlyTrustGate(g)
	}
}

// SetSandboxEscapeApprover propagates to all created agents.
func (o *Orchestrator) SetSandboxEscapeApprover(g sandbox.EscapeApprover) {
	if o.defaultAgent != nil {
		o.defaultAgent.SetSandboxEscapeApprover(g)
	}
	for _, a := range o.agents {
		a.SetSandboxEscapeApprover(g)
	}
}

// SetReasoningLanguage propagates to all created agents.
func (o *Orchestrator) SetReasoningLanguage(lang string) {
	if o.defaultAgent != nil {
		o.defaultAgent.SetReasoningLanguage(lang)
	}
	for _, a := range o.agents {
		a.SetReasoningLanguage(lang)
	}
}

// SetResponseLanguage propagates to all created agents.
func (o *Orchestrator) SetResponseLanguage(lang string) {
	if o.defaultAgent != nil {
		o.defaultAgent.SetResponseLanguage(lang)
	}
	for _, a := range o.agents {
		a.SetResponseLanguage(lang)
	}
}
