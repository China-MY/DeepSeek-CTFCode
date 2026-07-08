// Package agent defines AgentConfig, the multi-agent descriptor, and
// AgentRegistry which manages the set of configured agents. It works alongside
// the existing single-Agent and Coordinator runners — when no agents are
// configured, the system behaves exactly as before.
//
// Integration points:
//   - Config: loaded from the [[agents]] TOML section in reasonix.toml
//     (when absent, single-agent mode is preserved unchanged).
//   - Boot: the instantiating path (boot.Build or a frontend) calls
//     NewAgentRegistry to build the agent set, then wires an Orchestrator or
//     passes the registry to the handoff tools.
//   - Tool ACL: each AgentConfig.AllowedTools filters the shared tool.Registry
//     so an operator agent sees bash, a planner agent sees only reads + handoff.
//   - Handoff: AgentConfig.AllowedHandoffs gates which agents an agent may
//     transfer control to, enforced by the Orchestrator.
package agent

import (
	"fmt"
	"sort"
	"strings"
)

// ThinkingLevel controls how much reasoning budget a model uses.
type ThinkingLevel string

const (
	ThinkingLow    ThinkingLevel = "low"
	ThinkingMedium ThinkingLevel = "medium"
	ThinkingHigh   ThinkingLevel = "high"
)

// AgentConfig describes one agent role in a multi-agent session. An agent
// has its own identity (id/name/role), its own system prompt, a binding to a
// provider entry + model, and ACLs for which tools it may call and which other
// agents it may hand off to.
type AgentConfig struct {
	// ID is a short, unique identifier (e.g. "planner", "builder").
	// It is used as the target in handoff_to_agent calls.
	ID string `json:"id" toml:"id"`

	// Name is a human-readable label (e.g. "规划师", "执行者").
	Name string `json:"name" toml:"name"`

	// Role is a one-line description of the agent's function.
	Role string `json:"role" toml:"role"`

	// SystemPrompt is the core instruction prompt for this agent. When
	// empty, the session's shared system prompt is used.
	SystemPrompt string `json:"system_prompt" toml:"system_prompt"`


	// SystemPromptFile is path to a file with the system prompt.
	// Relative paths are resolved from the workspace root.
	// Takes precedence when SystemPrompt is empty.
	SystemPromptFile string `json:"system_prompt_file" toml:"system_prompt_file"`

	// Provider references the name of a configured [[providers]] entry.
	// When empty, the session's default provider is used.
	Provider string `json:"provider" toml:"provider"`

	// Model overrides the default model from the resolved provider.
	// Format: "model-name" or "" for the provider's default.
	Model string `json:"model" toml:"model"`

	// AllowedTools is the set of tool names this agent is permitted to call.
	// A single element "*" means all tools in the shared registry.
	// An empty list means no tools at all — the agent can only handoff or
	// complete. When not set (nil), all tools are allowed (backward compat).
	AllowedTools []string `json:"allowed_tools" toml:"allowed_tools"`

	// AllowedHandoffs lists agent IDs this agent may transfer control to.
	// An empty list means handoffs are forbidden. Not set (nil) means any.
	AllowedHandoffs []string `json:"allowed_handoffs" toml:"allowed_handoffs"`

	// ThinkingLevel controls the model's reasoning budget. Empty means
	// use the provider's default.
	ThinkingLevel string `json:"thinking_level" toml:"thinking_level"`

	// MountedSkills lists skill names this agent has access to. When non-empty,
	// only these skills are injected into the agent's system prompt index.
	// Empty means inherit the global skill index (all enabled skills).
	MountedSkills []string `json:"mounted_skills" toml:"mounted_skills"`

	// MountedKnowledge lists knowledge base entry names to inject into this
	// agent's context. Knowledge entries are Markdown documents stored in
	// .reasonix/knowledge/ directories. When non-empty, only the named entries
	// are loaded; when empty, no knowledge is mounted.
	MountedKnowledge []string `json:"mounted_knowledge" toml:"mounted_knowledge"`

	// EnableLearning enables automatic experience recording for this agent.
	// When true, the agent's key decisions and outcomes are persisted and can
	// be recalled in future sessions via the experience system.
	EnableLearning bool `json:"enable_learning" toml:"enable_learning"`
}

// ToolAllAllowed is the magic token that means "all tools".
const ToolAllAllowed = "*"

// AllowsAllTools reports whether this agent may call every tool in the
// shared registry.
func (a AgentConfig) AllowsAllTools() bool {
	for _, t := range a.AllowedTools {
		if t == ToolAllAllowed {
			return true
		}
	}
	return len(a.AllowedTools) == 0
}

// AllowsTool reports whether the named tool is permitted for this agent.
func (a AgentConfig) AllowsTool(name string) bool {
	if a.AllowsAllTools() {
		return true
	}
	for _, t := range a.AllowedTools {
		if t == name {
			return true
		}
	}
	return false
}

// MayHandoffTo reports whether this agent may hand off to the named agent.
func (a AgentConfig) MayHandoffTo(targetID string) bool {
	if len(a.AllowedHandoffs) == 0 {
		return false
	}
	for _, id := range a.AllowedHandoffs {
		if id == targetID {
			return true
		}
	}
	return false
}

// Validate checks that the config is internally consistent.
func (a AgentConfig) Validate() error {
	if strings.TrimSpace(a.ID) == "" {
		return fmt.Errorf("agent id must not be empty")
	}
	if a.AllowedTools == nil {
		// nil = allow all, which is fine.
	}
	for _, t := range a.AllowedTools {
		if strings.TrimSpace(t) == "" {
			return fmt.Errorf("agent %q: allowed_tools contains empty entry", a.ID)
		}
	}
	return nil
}

// AgentRegistry holds the set of configured agents for a multi-agent session.
// It is populated from configuration and queried by the Orchestrator and the
// handoff tools. When empty or containing only the implicit "default" agent,
// the session runs in single-agent mode.
type AgentRegistry struct {
	agents map[string]AgentConfig
	order  []string // insertion order for deterministic iteration
}

// NewAgentRegistry returns an empty registry.
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{agents: map[string]AgentConfig{}}
}

// Add registers an agent config. Duplicate IDs are replaced.
func (r *AgentRegistry) Add(cfg AgentConfig) {
	id := cfg.ID
	if _, ok := r.agents[id]; !ok {
		r.order = append(r.order, id)
	}
	r.agents[id] = cfg
}

// Get returns an agent config by ID.
func (r *AgentRegistry) Get(id string) (AgentConfig, bool) {
	cfg, ok := r.agents[id]
	return cfg, ok
}

// DefaultAgentID is the implicit single-agent ID when no agents are configured.
const DefaultAgentID = "default"

// DefaultAgent returns a minimal config for single-agent mode — it has no
// restrictions and serves as the fallback when the registry is empty.
func DefaultAgent() AgentConfig {
	return AgentConfig{
		ID:   DefaultAgentID,
		Name: "default",
		Role: "default coding agent",
	}
}

// List returns all registered agents in insertion order.
func (r *AgentRegistry) List() []AgentConfig {
	out := make([]AgentConfig, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.agents[id])
	}
	return out
}

// IDs returns the agent IDs in insertion order.
func (r *AgentRegistry) IDs() []string {
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// Len returns the number of registered agents.
func (r *AgentRegistry) Len() int {
	return len(r.agents)
}

// IsMultiAgent reports whether the registry has more than one distinct agent
// (beyond the implicit default).
func (r *AgentRegistry) IsMultiAgent() bool {
	if r == nil || len(r.agents) == 0 {
		return false
	}
	return len(r.agents) > 1 || (len(r.agents) == 1 && r.order[0] != DefaultAgentID)
}

// AgentNames returns the sorted display names of all registered agents.
func (r *AgentRegistry) AgentNames() []string {
	names := make([]string, 0, len(r.agents))
	for _, cfg := range r.agents {
		name := strings.TrimSpace(cfg.Name)
		if name == "" {
			name = cfg.ID
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ValidateAll checks every registered agent and returns all errors.
func (r *AgentRegistry) ValidateAll() error {
	var errs []string
	for _, cfg := range r.agents {
		if err := cfg.Validate(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("agent registry validation errors:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
