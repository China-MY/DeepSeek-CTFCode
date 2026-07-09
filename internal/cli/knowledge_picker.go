package cli

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"ctfcode/internal/knowledge"
)

// knowledgePicker is a modal overlay for browsing knowledge entries.
type knowledgePicker struct {
	entries     []knowledge.Entry
	filtered    []knowledge.Entry
	query       string
	sel         int
	searchMode  bool
	detailEntry *knowledge.Entry // non-nil when viewing detail
}

func (m *chatTUI) openKnowledgePicker() {
	store := m.knowledgeStore()
	if store == nil {
		m.notice("knowledge store not available")
		return
	}
	entries := store.List()
	if len(entries) == 0 {
		m.notice("no knowledge entries found")
		return
	}
	m.knowledgePick = &knowledgePicker{
		entries:  entries,
		filtered: entries,
	}
}

func (m chatTUI) handleKnowledgePickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	p := m.knowledgePick
	if p == nil {
		return m, nil
	}

	if p.detailEntry != nil {
		return m.handleKnowledgePickerDetailKey(msg)
	}

	if p.searchMode {
		switch msg.String() {
		case "esc":
			p.searchMode = false
			return m, nil
		case "enter":
			m.knowledgePick = nil
			return m, nil
		case "backspace":
			if len(p.query) > 0 {
				p.query = p.query[:len(p.query)-1]
				p.applyFilter()
			}
			return m, nil
		case "up", "k":
			if p.sel > 0 {
				p.sel--
			}
			return m, nil
		case "down", "j":
			if p.sel < len(p.filtered)-1 {
				p.sel++
			}
			return m, nil
		default:
			if t := msg.Text; t != "" {
				p.query += t
			} else if s := msg.String(); len(s) == 1 && s[0] >= 32 && s[0] < 127 {
				p.query += s
			}
			p.applyFilter()
			p.sel = clampSel(p.sel, p.filtered)
			return m, nil
		}
	}

	switch msg.String() {
	case "esc":
		m.knowledgePick = nil
	case "up", "k":
		if p.sel > 0 {
			p.sel--
		}
	case "down", "j":
		if p.sel < len(p.filtered)-1 {
			p.sel++
		}
	case "enter", "right", "l":
		if len(p.filtered) > 0 && p.sel < len(p.filtered) {
			p.detailEntry = &p.filtered[p.sel]
		}
	case "/":
		p.searchMode = true
		p.query = ""
		p.applyFilter()
	}

	return m, nil
}

func (m chatTUI) handleKnowledgePickerDetailKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	p := m.knowledgePick
	switch msg.String() {
	case "esc", "left", "h":
		p.detailEntry = nil
	}
	return m, nil
}

func (p *knowledgePicker) applyFilter() {
	if p.query == "" {
		p.filtered = p.entries
		return
	}
	q := strings.ToLower(p.query)
	var out []knowledge.Entry
	for _, e := range p.entries {
		if strings.Contains(strings.ToLower(e.Name), q) ||
			strings.Contains(strings.ToLower(e.Description), q) {
			out = append(out, e)
		}
	}
	p.filtered = out
	if p.sel >= len(p.filtered) {
		p.sel = len(p.filtered) - 1
	}
	if p.sel < 0 {
		p.sel = 0
	}
}

// renderKnowledgePicker renders the knowledge browser overlay.
func (m chatTUI) renderKnowledgePicker() string {
	p := m.knowledgePick
	if p == nil {
		return ""
	}
	w := max(viewWidth(m.width), 40)

	if p.detailEntry != nil {
		return managerContentPanelStyle(w).Render(
			renderKnowledgeShow(m.width, *p.detailEntry))
	}

	var b strings.Builder
	title := fmt.Sprintf("knowledge (%d)", len(p.filtered))
	if p.searchMode {
		title += "  /" + p.query + "█"
	}
	fmt.Fprintf(&b, "%s\n", viewHeader(title))
	for i, e := range p.filtered {
		mark := "  "
		if i == p.sel {
			mark = "▸ "
		}
		used := 2 + viewPadWidth(e.Name, 22) + 2
		desc := viewCompactText(e.Description, viewBudget(w, used))
		fmt.Fprintf(&b, "%s%-22s %s\n", mark, e.Name, desc)
	}
	return managerContentPanelStyle(w).Render(strings.TrimRight(b.String(), "\n"))
}

func (m chatTUI) knowledgePickerFooterHint() string {
	if m.knowledgePick == nil {
		return ""
	}
	if m.knowledgePick.detailEntry != nil {
		return "←/Esc back"
	}
	if m.knowledgePick.searchMode {
		return "type to filter · ⏎ close · Esc cancel"
	}
	return "↑/↓ navigate · →/⏎ view · / search · Esc close"
}
