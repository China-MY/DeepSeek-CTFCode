package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ctfcode/internal/knowledge"
)

const knowledgeShowMaxLines = 80

func renderKnowledgeList(width int, entries []knowledge.Entry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", viewHeader("knowledge (%d)", len(entries)))
	if len(entries) == 0 {
		fmt.Fprintf(&b, "  %s\n", viewMeta("(none)"))
		b.WriteString(viewHint("add .md files under .ctfcode/knowledge/ or knowledge_base/"))
		return strings.TrimRight(b.String(), "\n")
	}
	for _, e := range entries {
		name := e.Name
		used := 2 + viewPadWidth(name, 22) + 2
		desc := viewCompactText(e.Description, viewBudget(width, used))
		tags := ""
		if len(e.Tags) > 0 {
			tags = "  " + viewMeta(strings.Join(e.Tags, ", "))
		}
		fmt.Fprintf(&b, "  %-22s %s%s\n", name, desc, tags)
	}
	b.WriteString(viewHint("view: /knowledge show <name> · search: /knowledge search <query>"))
	return strings.TrimRight(b.String(), "\n")
}

func renderKnowledgeShow(width int, e knowledge.Entry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", viewHeader("knowledge:"), viewCompactText(e.Name, viewBudget(width, 12)))
	if strings.TrimSpace(e.Description) != "" {
		fmt.Fprintf(&b, "  %s\n", viewCompactText(e.Description, viewBudget(width, 2)))
	}
	if len(e.Tags) > 0 {
		fmt.Fprintf(&b, "  %s\n", viewMeta(strings.Join(e.Tags, ", ")))
	}
	if len(e.Agents) > 0 {
		fmt.Fprintf(&b, "  %s\n", viewMeta("agents: "+strings.Join(e.Agents, ", ")))
	}
	if strings.TrimSpace(e.Path) != "" {
		fmt.Fprintf(&b, "  %s\n", viewMeta(viewCompactPath(e.Path, viewBudget(width, 2))))
	}
	body, extra := viewBodyPreview(e.Body, knowledgeShowMaxLines)
	if strings.TrimSpace(body) != "" {
		b.WriteString("\n")
		b.WriteString(viewProtectLines(body, width))
	}
	if extra > 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(viewMore(extra, "lines"))
	}
	return strings.TrimRight(b.String(), "\n")
}

// buildKnowledgeStore creates a knowledge store that scans the project's
// .ctfcode/knowledge/, knowledge_base/ (for markdown refs), and global paths.
func buildKnowledgeStore() *knowledge.Store {
	cwd, _ := os.Getwd()
	var customPaths []string

	// Add knowledge_base/ if it exists (contains reference markdown knowledge).
	if cwd != "" {
		kb := filepath.Join(cwd, "knowledge_base")
		if fi, err := os.Stat(kb); err == nil && fi.IsDir() {
			customPaths = append(customPaths, kb)
		}
	}

	// Also scan .ctfcode/knowledge/ for project CTF knowledge.
	if cwd != "" {
		ctfKnowledge := filepath.Join(cwd, ".ctfcode", "knowledge")
		if fi, err := os.Stat(ctfKnowledge); err == nil && fi.IsDir() {
			customPaths = append(customPaths, ctfKnowledge)
		}
	}

	return knowledge.Load(knowledge.LoadOptions{
		ProjectRoot: cwd,
		CustomPaths: customPaths,
	})
}

// knowledgeSearch returns entries whose name, description, or body contains the query.
func knowledgeSearch(store *knowledge.Store, query string) []knowledge.Entry {
	if query == "" {
		return store.List()
	}
	q := strings.ToLower(query)
	var out []knowledge.Entry
	for _, e := range store.List() {
		if strings.Contains(strings.ToLower(e.Name), q) ||
			strings.Contains(strings.ToLower(e.Description), q) ||
			strings.Contains(strings.ToLower(e.Body), q) {
			out = append(out, e)
		}
	}
	return out
}
