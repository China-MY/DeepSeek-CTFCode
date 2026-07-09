package cli

import (
	"strings"

	"ctfcode/internal/knowledge"
)

// runKnowledgeSubcommand handles the /knowledge slash command.
func (m *chatTUI) runKnowledgeSubcommand(input string) {
	args := tokenizeArgs(input)
	sub := ""
	if len(args) > 1 {
		sub = strings.ToLower(args[1])
	}
	switch sub {
	case "", "list", "ls":
		m.knowledgeList()
	case "show", "cat":
		if len(args) < 3 {
			m.notice("usage: /knowledge show <name>")
			return
		}
		m.knowledgeShow(args[2])
	case "search", "find":
		if len(args) < 3 {
			m.notice("usage: /knowledge search <query>")
			return
		}
		m.knowledgeSearch(strings.Join(args[2:], " "))
	case "browse":
		m.openKnowledgePicker()
	default:
		hint := ""
		store := m.knowledgeStore()
		if store != nil {
			if _, ok := store.Get(args[1]); ok {
				hint = " (to view it, type /knowledge show " + args[1] + ")"
			}
		}
		m.notice("unknown /knowledge subcommand " + args[1] + hint +
			" — try: /knowledge, /knowledge list, /knowledge show <name>, /knowledge search <query>, /knowledge browse")
	}
}

func (m *chatTUI) knowledgeList() {
	store := m.knowledgeStore()
	if store == nil {
		m.notice("knowledge store not available")
		return
	}
	entries := store.List()
	m.commitLine(renderKnowledgeList(m.width, entries))
}

func (m *chatTUI) knowledgeShow(name string) {
	store := m.knowledgeStore()
	if store == nil {
		m.notice("knowledge store not available")
		return
	}
	e, ok := store.Get(name)
	if !ok {
		m.notice("unknown knowledge: " + name)
		return
	}
	m.commitLine(renderKnowledgeShow(m.width, e))
}

func (m *chatTUI) knowledgeSearch(query string) {
	store := m.knowledgeStore()
	if store == nil {
		m.notice("knowledge store not available")
		return
	}
	results := knowledgeSearch(store, query)
	if len(results) == 0 {
		m.notice("no knowledge entries match: " + query)
		return
	}
	m.commitLine(renderKnowledgeList(m.width, results))
}

// knowledgeStore returns a lazily-built knowledge store.
func (m *chatTUI) knowledgeStore() *knowledge.Store {
	return buildKnowledgeStore()
}
