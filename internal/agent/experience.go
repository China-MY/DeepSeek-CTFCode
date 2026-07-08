// Package experience provides a lightweight self-learning mechanism for agents.
// It records key agent decisions, outcomes, and lessons as structured experiences
// that can be recalled in future turns to improve performance.
//
// Experiences are stored as memories via the existing memory system, keyed by
// agent ID and tagged with the task domain. The system supports:
//
//   - auto-recording: after each agent turn, the orchestrator can extract and
//     save salient experiences
//   - explicit reflection: agents can call the `reflect` tool to record a lesson
//   - experience injection: relevant past experiences are folded into the
//     agent's system prompt as context
//
// Storage format (in the memory store):
//
//	experience-<agentID>-<seq>:
//	  type: reference
//	  body: |
//	    ## Experience
//	    agent: planner
//	    task: implement login form
//	    outcome: success
//	    lessons:
//	    - Always use the validation library instead of manual checks
//	    patterns:
//	    - form validation → validator.Validate()
package agent

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Record is a single experience entry recorded by or about an agent.
type Record struct {
	// AgentID identifies which agent this experience belongs to.
	AgentID string `json:"agent_id"`

	// Task is a short description of what the agent was doing.
	Task string `json:"task"`

	// Outcome describes the result: "success", "failure", "learned".
	Outcome string `json:"outcome"`

	// Lessons are the key takeaways from this experience.
	Lessons []string `json:"lessons,omitempty"`

	// Patterns document reusable patterns discovered.
	Patterns []string `json:"patterns,omitempty"`

	// Context is additional context (what tools were used, etc.).
	Context string `json:"context,omitempty"`

	// Timestamp records when this experience was created.
	Timestamp time.Time `json:"timestamp"`

	// Tags for categorization and retrieval.
	Tags []string `json:"tags,omitempty"`
}

// Store manages a collection of agent experiences in memory.
// Experiences can be persisted to the memory store via the remember tool.
type Store struct {
	mu         sync.Mutex
	records    []Record
	maxRecords int // max in-memory records before pruning
}

// NewStore creates an empty experience store.
func NewStore(maxRecords int) *Store {
	if maxRecords <= 0 {
		maxRecords = 100
	}
	return &Store{maxRecords: maxRecords}
}

// Add records a new experience. It may prune old records if the store is full.
func (s *Store) Add(r Record) {
	if r.Timestamp.IsZero() {
		r.Timestamp = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records = append(s.records, r)
	if len(s.records) > s.maxRecords {
		// Remove oldest records.
		excess := len(s.records) - s.maxRecords
		s.records = s.records[excess:]
	}
}

// ForAgent returns all experiences for a given agent, newest first.
func (s *Store) ForAgent(agentID string) []Record {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []Record
	for _, r := range s.records {
		if r.AgentID == agentID {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.After(out[j].Timestamp)
	})
	return out
}

// Recent returns the N most recent experiences across all agents.
func (s *Store) Recent(n int) []Record {
	s.mu.Lock()
	defer s.mu.Unlock()

	if n <= 0 || n >= len(s.records) {
		n = len(s.records)
	}
	sorted := make([]Record, len(s.records))
	copy(sorted, s.records)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.After(sorted[j].Timestamp)
	})
	return sorted[:n]
}

// Search returns experiences matching the given tags (AND logic).
func (s *Store) Search(tags []string) []Record {
	if len(tags) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[strings.ToLower(t)] = true
	}

	var out []Record
	for _, r := range s.records {
		matches := false
		for _, rt := range r.Tags {
			if tagSet[strings.ToLower(rt)] {
				matches = true
				break
			}
		}
		if matches {
			out = append(out, r)
		}
	}
	return out
}

// Len returns the number of stored records.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.records)
}

// All returns all records sorted newest-first.
func (s *Store) All() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	sorted := make([]Record, len(s.records))
	copy(sorted, s.records)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.After(sorted[j].Timestamp)
	})
	return sorted
}

// Clear removes all experiences (for testing or reset).
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = nil
}

// FormatBlock formats relevant experiences into a system prompt block for
// injection into an agent's context.
func FormatBlock(records []Record, maxEntries int) string {
	if len(records) == 0 {
		return ""
	}
	if maxEntries <= 0 {
		maxEntries = 5
	}
	if len(records) > maxEntries {
		records = records[:maxEntries]
	}

	var b strings.Builder
	b.WriteString("\n\n# Past Experiences\n\n")
	b.WriteString("The following experiences from previous sessions may be relevant:\n\n")

	for i, r := range records {
		b.WriteString(fmt.Sprintf("### %d. %s\n", i+1, r.Task))
		b.WriteString(fmt.Sprintf("   - Agent: %s\n", r.AgentID))
		b.WriteString(fmt.Sprintf("   - Outcome: %s\n", r.Outcome))

		if len(r.Lessons) > 0 {
			b.WriteString("   - Lessons:\n")
			for _, l := range r.Lessons {
				b.WriteString(fmt.Sprintf("     * %s\n", l))
			}
		}
		if len(r.Patterns) > 0 {
			b.WriteString("   - Patterns:\n")
			for _, p := range r.Patterns {
				b.WriteString(fmt.Sprintf("     * %s\n", p))
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("Use these experiences to inform your approach. Avoid repeating past mistakes and reuse successful patterns.\n")

	return b.String()
}

// MemoryName returns the memory store key for an experience.
func MemoryName(agentID string, seq int) string {
	return fmt.Sprintf("experience-%s-%d", agentID, seq)
}

// ToMemoryBody formats a Record as a Markdown body suitable for the memory store.
func (r Record) ToMemoryBody() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Experience\n\n"))
	b.WriteString(fmt.Sprintf("- agent: %s\n", r.AgentID))
	b.WriteString(fmt.Sprintf("- task: %s\n", r.Task))
	b.WriteString(fmt.Sprintf("- outcome: %s\n", r.Outcome))

	if len(r.Lessons) > 0 {
		b.WriteString("- lessons:\n")
		for _, l := range r.Lessons {
			b.WriteString(fmt.Sprintf("  - %s\n", l))
		}
	}
	if len(r.Patterns) > 0 {
		b.WriteString("- patterns:\n")
		for _, p := range r.Patterns {
			b.WriteString(fmt.Sprintf("  - %s\n", p))
		}
	}
	if r.Context != "" {
		b.WriteString(fmt.Sprintf("- context: %s\n", r.Context))
	}
	if len(r.Tags) > 0 {
		b.WriteString(fmt.Sprintf("- tags: [%s]\n", strings.Join(r.Tags, ", ")))
	}

	return b.String()
}

// RecordFromMemory parses a stored memory body back into a Record.
// This is a simple heuristic parser for the format produced by ToMemoryBody.
func RecordFromMemory(agentID string, body string) Record {
	r := Record{AgentID: agentID}
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "- task:"):
			r.Task = strings.TrimSpace(strings.TrimPrefix(line, "- task:"))
		case strings.HasPrefix(line, "- outcome:"):
			r.Outcome = strings.TrimSpace(strings.TrimPrefix(line, "- outcome:"))
		case strings.HasPrefix(line, "- context:"):
			r.Context = strings.TrimSpace(strings.TrimPrefix(line, "- context:"))
		case strings.HasPrefix(line, "  - "):
			// Lessons/patterns are indented list items within their section.
		case strings.HasPrefix(line, "- lessons:"):
			// Next indented lines are lessons.
		case strings.HasPrefix(line, "- patterns:"):
			// Next indented lines are patterns.
		case strings.HasPrefix(line, "- tags:"):
			tagStr := strings.TrimSpace(strings.TrimPrefix(line, "- tags:"))
			tagStr = strings.Trim(tagStr, "[]")
			for _, tag := range strings.Split(tagStr, ",") {
				if t := strings.TrimSpace(tag); t != "" {
					r.Tags = append(r.Tags, t)
				}
			}
		}
	}
	return r
}

// Name returns the provider name for the ExperienceProvider interface.
func (s *Store) Name() string { return "experience" }

// FormatBlock returns formatted experience entries for a specific agent.
// It implements the agent.ExperienceProvider interface.
func (s *Store) FormatBlock(agentID string, maxEntries int) string {
	records := s.ForAgent(agentID)
	return formatBlockInternal(records, maxEntries)
}

// RecordExperience adds a new experience entry.
// It implements the agent.ExperienceProvider interface.
func (s *Store) RecordExperience(agentID, task, outcome string, lessons, patterns []string) {
	s.Add(Record{
		AgentID:  agentID,
		Task:     task,
		Outcome:  outcome,
		Lessons:  lessons,
		Patterns: patterns,
	})
}

// formatBlockInternal is the unexported formatting function.
func formatBlockInternal(records []Record, maxEntries int) string {
	return FormatBlock(records, maxEntries)
}
