// Package knowledge provides a Markdown-based knowledge base system that can be
// mounted per-agent. Each knowledge entry is a Markdown file (guide, reference,
// or playbook) stored under a project's .reasonix/knowledge/ directory or a
// custom path. Agents reference knowledge by name; the system loads the content
// and injects it into the agent's system prompt or session context.
//
// Directory layout:
//
//	.reasonix/knowledge/
//	  general/
//	    project-rules.md       # 项目规则与约定
//	    architecture.md         # 系统架构说明
//	  security/
//	    secure-coding.md       # 安全编码规范
//	    threat-model.md         # 威胁模型
//	  agents/
//	    planner/
//	      planning-guide.md     # 仅 Planner agent 可见
//	    operator/
//	      deploy-guide.md       # 仅 Operator agent 可见
//
// Knowledge files are plain Markdown. Frontmatter (--- delimited) can set:
//
//	name: custom_name        # override filename as the reference name
//	description: ...         # one-line summary shown in index
//	tags: [tag1, tag2]       # for filtering/matching
//	agents: [planner]        # restrict which agents see this file
package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Entry is a single knowledge document loaded from disk.
type Entry struct {
	// Name is the canonical reference name (filename stem or frontmatter name).
	Name string `json:"name"`
	// Description is a one-line summary.
	Description string `json:"description,omitempty"`
	// Tags for categorization and filtering.
	Tags []string `json:"tags,omitempty"`
	// Agents restricts visibility to named agents. Empty means all agents.
	Agents []string `json:"agents,omitempty"`
	// Body is the Markdown content.
	Body string `json:"body"`
	// Path is the source file path.
	Path string `json:"path"`
}

// Store manages a collection of knowledge entries.
type Store struct {
	entries map[string]Entry
	order   []string // insertion order for deterministic iteration
}

// NewStore creates an empty knowledge store.
func NewStore() *Store {
	return &Store{entries: map[string]Entry{}}
}

// Add registers a knowledge entry. Duplicate names are replaced.
func (s *Store) Add(e Entry) {
	if _, ok := s.entries[e.Name]; !ok {
		s.order = append(s.order, e.Name)
	}
	s.entries[e.Name] = e
}

// Get returns a knowledge entry by name.
func (s *Store) Get(name string) (Entry, bool) {
	e, ok := s.entries[name]
	return e, ok
}

// List returns all entries in insertion order.
func (s *Store) List() []Entry {
	out := make([]Entry, 0, len(s.order))
	for _, name := range s.order {
		out = append(out, s.entries[name])
	}
	return out
}

// ForAgent returns knowledge entries visible to the named agent.
func (s *Store) ForAgent(agentID string) []Entry {
	var out []Entry
	for _, name := range s.order {
		e := s.entries[name]
		if len(e.Agents) == 0 {
			out = append(out, e)
			continue
		}
		for _, a := range e.Agents {
			if a == agentID {
				out = append(out, e)
				break
			}
		}
	}
	return out
}

// Names returns all entry names.
func (s *Store) Names() []string {
	out := make([]string, len(s.order))
	copy(out, s.order)
	return out
}

// Len returns the number of entries.
func (s *Store) Len() int {
	return len(s.entries)
}

// LoadOptions configures knowledge discovery.
type LoadOptions struct {
	// ProjectRoot is the project root for discovering .reasonix/knowledge/.
	ProjectRoot string
	// CustomPaths are additional directories to scan.
	CustomPaths []string
	// ExcludedPaths are directories to skip.
	ExcludedPaths []string
}

// Load discovers and loads all knowledge entries from the configured paths.
// It scans:
//  1. <project>/.reasonix/knowledge/
//  2. Custom paths (if any)
//  3. ~/.reasonix/knowledge/ (global user knowledge)
func Load(opts LoadOptions) *Store {
	s := NewStore()

	paths := orderedSearchPaths(opts.ProjectRoot, opts.CustomPaths)
	seen := map[string]bool{} // deduplicate by name

	for _, dir := range paths {
		if dir == "" {
			continue
		}
		if isExcluded(dir, opts.ExcludedPaths) {
			continue
		}
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		entries := scanDir(dir, seen)
		for _, e := range entries {
			s.Add(e)
		}
	}

	return s
}

// orderedSearchPaths returns the search paths in priority order (highest first).
func orderedSearchPaths(projectRoot string, customPaths []string) []string {
	var paths []string

	// 1. Project knowledge (.reasonix/knowledge/)
	if projectRoot != "" {
		paths = append(paths, filepath.Join(projectRoot, ".reasonix", "knowledge"))
	}

	// 2. Custom paths
	paths = append(paths, customPaths...)

	// 3. Global user knowledge (~/.reasonix/knowledge/)
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".reasonix", "knowledge"))
	}

	return paths
}

func isExcluded(dir string, excluded []string) bool {
	for _, e := range excluded {
		if strings.TrimSpace(e) == "" {
			continue
		}
		if abs, err := filepath.Abs(e); err == nil && abs == dir {
			return true
		}
		if dir == e {
			return true
		}
	}
	return false
}

// scanDir walks a directory tree and loads all Markdown knowledge files.
func scanDir(root string, seen map[string]bool) []Entry {
	var entries []Entry

	filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible
		}
		if fi.IsDir() {
			// Skip hidden directories and common noise.
			if strings.HasPrefix(fi.Name(), ".") && fi.Name() != "." {
				return filepath.SkipDir
			}
			if fi.Name() == "node_modules" || fi.Name() == "assets" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only .md files.
		if !strings.HasSuffix(strings.ToLower(fi.Name()), ".md") {
			return nil
		}

		name := strings.TrimSuffix(fi.Name(), ".md")
		if seen[name] {
			return nil // deduplicate
		}
		seen[name] = true

		e := loadFile(path, name)
		if e != nil {
			entries = append(entries, *e)
		}
		return nil
	})

	return entries
}

// loadFile reads a single Markdown knowledge file and parses its frontmatter.
func loadFile(path string, defaultName string) *Entry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	body := string(data)
	e := &Entry{
		Name: defaultName,
		Body: body,
		Path: path,
	}

	// Parse frontmatter: ---\n...\n---
	if strings.HasPrefix(body, "---") {
		rest := body[3:]
		if idx := strings.Index(rest, "\n---"); idx >= 0 {
			fm := rest[:idx]
			e.Body = strings.TrimSpace(rest[idx+4:])
			parseFrontmatter(fm, e)
		}
	}

	return e
}

// parseFrontmatter extracts metadata from YAML-ish frontmatter.
func parseFrontmatter(fm string, e *Entry) {
	lines := strings.Split(fm, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		switch {
		case strings.HasPrefix(line, "name:"):
			e.Name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))

		case strings.HasPrefix(line, "description:"):
			e.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))

		case strings.HasPrefix(line, "tags:"):
			tagStr := strings.TrimSpace(strings.TrimPrefix(line, "tags:"))
			tagStr = strings.Trim(tagStr, "[]")
			for _, tag := range strings.Split(tagStr, ",") {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					e.Tags = append(e.Tags, tag)
				}
			}

		case strings.HasPrefix(line, "agents:"):
			agentStr := strings.TrimSpace(strings.TrimPrefix(line, "agents:"))
			agentStr = strings.Trim(agentStr, "[]")
			for _, a := range strings.Split(agentStr, ",") {
				a = strings.TrimSpace(a)
				if a != "" {
					e.Agents = append(e.Agents, a)
				}
			}
		}
	}
}

// MountBlock formats knowledge entries into a system prompt block.
func MountBlock(entries []Entry) string {
	if len(entries) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\n# Mounted Knowledge\n\n")
	b.WriteString("The following knowledge documents are available for reference:\n\n")

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	for _, e := range entries {
		desc := e.Description
		if desc != "" {
			desc = " — " + desc
		}
		b.WriteString(fmt.Sprintf("- **%s**%s\n", e.Name, desc))
	}

	// Inject full content for mounted entries.
	b.WriteString("\n")
	for _, e := range entries {
		b.WriteString(fmt.Sprintf("### %s\n\n", e.Name))
		if e.Description != "" {
			b.WriteString(fmt.Sprintf("> %s\n\n", e.Description))
		}
		b.WriteString(e.Body)
		b.WriteString("\n\n")
	}

	return b.String()
}

// Name returns the provider name for the KnowledgeProvider interface.
func (s *Store) Name() string { return "knowledge" }

// MountBlock returns formatted knowledge entries for a specific agent.
// It implements the agent.KnowledgeProvider interface.
func (s *Store) MountBlock(agentID string, mountedNames []string) string {
	if len(mountedNames) == 0 {
		return ""
	}
	var entries []Entry
	for _, name := range mountedNames {
		if e, ok := s.Get(name); ok {
			entries = append(entries, e)
		}
	}
	if len(entries) == 0 {
		// Try agent-scoped lookup: if no exact name match, use ForAgent.
		entries = s.ForAgent(agentID)
	}
	return MountBlock(entries)
}
