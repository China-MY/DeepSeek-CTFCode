package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ctfcode/internal/knowledge"
	"ctfcode/internal/tool"
)

func init() { tool.RegisterBuiltin(knowledgeSearchTool{}) }

type knowledgeSearchTool struct{}

func (knowledgeSearchTool) Name() string { return "search_knowledge" }

func (knowledgeSearchTool) Description() string {
	return `Search the local knowledge base and CTF exploit reference library for techniques, payloads, and exploits matching a query. 
Searches across knowledge_base/ (PayloadsAllTheThings, exploitarium PoCs, CVE labs) and .ctfcode/knowledge/ (recon guides, exploit playbooks). 
Use this when you identify a target technology, service, port, or CVE and need relevant attack techniques. 
Returns matching file paths with content snippets ordered by relevance.`
}

func (knowledgeSearchTool) Schema() json.RawMessage {
	return json.RawMessage(`{
"type":"object",
"properties":{
  "query":{"type":"string","description":"What to search for — e.g. a technology name (\"ThinkPHP\", \"Shiro\"), CVE ID (\"CVE-2026-25243\"), service (\"Redis\", \"MySQL\"), or vulnerability type (\"SQL injection\", \"deserialization\", \"SSRF\"). Be specific for best results."},
  "max_results":{"type":"integer","description":"Maximum number of results to return (default 10, max 30)","default":10}
},
"required":["query"]
}`)
}

func (knowledgeSearchTool) ReadOnly() bool { return true }

func (knowledgeSearchTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Query     string `json:"query"`
		MaxResults int   `json:"max_results"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("search_knowledge: invalid args: %w", err)
	}
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return "", fmt.Errorf("search_knowledge: query must not be empty")
	}
	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = 10
	}
	if maxResults > 30 {
		maxResults = 30
	}

	cwd, err := os.Getwd()
	if err != nil {
		// Fall back to empty cwd; knowledge.Load handles empty ProjectRoot.
		cwd = ""
	}

	var customPaths []string

	// Scan knowledge_base/ for reference exploit content.
	if cwd != "" {
		kb := filepath.Join(cwd, "knowledge_base")
		if fi, err := os.Stat(kb); err == nil && fi.IsDir() {
			customPaths = append(customPaths, kb)
		}
	}

	// Scan .ctfcode/knowledge/ for project CTF knowledge.
	if cwd != "" {
		ctfKb := filepath.Join(cwd, ".ctfcode", "knowledge")
		if fi, err := os.Stat(ctfKb); err == nil && fi.IsDir() {
			customPaths = append(customPaths, ctfKb)
		}
	}

	store := knowledge.Load(knowledge.LoadOptions{
		ProjectRoot: cwd,
		CustomPaths: customPaths,
	})

	if store.Len() == 0 {
		return "search_knowledge: no knowledge base found (checked knowledge_base/ and .ctfcode/knowledge/)", nil
	}

	// Search across all entries.
	q := strings.ToLower(query)
	type match struct {
		Name        string
		Description string
		Path        string
		Snippet     string
		Score       int
	}

	var matches []match
	for _, e := range store.List() {
		nameLower := strings.ToLower(e.Name)
		descLower := strings.ToLower(e.Description)
		bodyLower := strings.ToLower(e.Body)

		score := 0
		if strings.Contains(nameLower, q) {
			score += 10 // Name match = high relevance
		}
		if strings.Contains(descLower, q) {
			score += 5 // Description match
		}
		if strings.Contains(bodyLower, q) {
			score += 3 // Body content match
		}

		if score > 0 {
			// Extract a snippet around the first match in the body.
			snippet := extractSnippet(e.Body, q, 200)
			matches = append(matches, match{
				Name:        e.Name,
				Description: e.Description,
				Path:        e.Path,
				Snippet:     snippet,
				Score:       score,
			})
		}
	}

	// Sort by score descending.
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Score > matches[i].Score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	if len(matches) > maxResults {
		matches = matches[:maxResults]
	}

	if len(matches) == 0 {
		return fmt.Sprintf("search_knowledge: no results for %q — try a different search term or check the target's technology more precisely", query), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Knowledge base results for %q (%d found):\n\n", query, len(matches))
	for i, m := range matches {
		path := m.Path
		if cwd != "" && strings.HasPrefix(path, cwd) {
			path = "." + path[len(cwd):]
		}
		fmt.Fprintf(&b, "%d. %s", i+1, m.Name)
		if m.Description != "" {
			fmt.Fprintf(&b, " — %s", m.Description)
		}
		fmt.Fprintf(&b, "\n   📍 %s\n", path)
		if m.Snippet != "" {
			fmt.Fprintf(&b, "   %s\n", m.Snippet)
		}
		b.WriteString("\n")
	}
	b.WriteString("Use read_file on the file paths above to read the full content.")

	return strings.TrimRight(b.String(), "\n"), nil
}

// extractSnippet returns a short snippet around the first occurrence of query in body.
func extractSnippet(body, query string, maxLen int) string {
	if body == "" {
		return ""
	}
	q := strings.ToLower(query)
	bodyLower := strings.ToLower(body)
	idx := strings.Index(bodyLower, q)
	if idx < 0 {
		// No match in body, return first maxLen chars.
		if len(body) > maxLen {
			return body[:maxLen] + "..."
		}
		return body
	}

	// Start ~50 chars before the match, end at maxLen.
	start := idx - 50
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(body) {
		end = len(body)
	}
	snippet := body[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(body) {
		snippet = snippet + "..."
	}
	return snippet
}
