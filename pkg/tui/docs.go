package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/clcollins/srepd/pkg/docs"
)

const defaultDocsTabsPerPage = 8

func newDocsViewer() viewport.Model {
	return viewport.New(100, 100)
}

func docsTabPage(activeTab, tabsPerPage int) int {
	return activeTab / tabsPerPage
}

func docsPageStartEnd(activeTab, tabsPerPage, totalTabs int) (int, int) {
	page := docsTabPage(activeTab, tabsPerPage)
	start := page * tabsPerPage
	end := start + tabsPerPage
	if end > totalTabs {
		end = totalTabs
	}
	return start, end
}

func docsTabLabel(title string) string {
	return docs.TruncateTitle(title, 8)
}

func docsNextTab(current, total int) int {
	if total <= 1 {
		return 0
	}
	return (current + 1) % total
}

func docsPrevTab(current, total int) int {
	if total <= 1 {
		return 0
	}
	return (current + total - 1) % total
}

func docsPagingIndicator(activeTab, tabsPerPage, total int) string {
	totalPages := (total + tabsPerPage - 1) / tabsPerPage
	if totalPages <= 1 {
		return ""
	}
	currentPage := docsTabPage(activeTab, tabsPerPage) + 1
	return fmt.Sprintf(" [%d/%d]", currentPage, totalPages)
}

func buildDocsPageLabels(pages []docs.Doc, activeTab, tabsPerPage int) []string {
	start, end := docsPageStartEnd(activeTab, tabsPerPage, len(pages))
	labels := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		labels = append(labels, docsTabLabel(pages[i].Title))
	}
	return labels
}

func (m *model) clearDocsView() {
	m.viewingDocs = false
	m.docsActiveTab = 0
}

func (m model) renderDocsTabBar() string {
	if len(m.docsPages) == 0 {
		return ""
	}

	labels := buildDocsPageLabels(m.docsPages, m.docsActiveTab, m.docsTabsPerPage)
	start, _ := docsPageStartEnd(m.docsActiveTab, m.docsTabsPerPage, len(m.docsPages))

	var renderedTabs []string
	for i, label := range labels {
		globalIdx := start + i
		var style lipgloss.Style
		isFirst := i == 0
		isActive := globalIdx == m.docsActiveTab
		if isActive {
			style = m.styles.ActiveTab
		} else {
			style = m.styles.InactiveTab
		}
		border, _, _, _, _ := style.GetBorder()
		if isFirst && isActive {
			border.BottomLeft = "│"
		} else if isFirst && !isActive {
			border.BottomLeft = "├"
		}
		style = style.Border(border)
		renderedTabs = append(renderedTabs, style.Render(label))
	}

	tabRow := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
	tabWidth := lipgloss.Width(tabRow)
	indicator := docsPagingIndicator(m.docsActiveTab, m.docsTabsPerPage, len(m.docsPages))
	remainingWidth := windowSize.Width - tabWidth - 1
	if remainingWidth > 0 {
		gapContent := indicator
		if gapContent == "" {
			gapContent = " "
		}
		gapBorder := lipgloss.Border{Bottom: "─", Right: " ", BottomRight: "┐"}
		gap := lipgloss.NewStyle().
			BorderForeground(m.theme.Border).
			Border(gapBorder, false, true, true, false).
			Width(remainingWidth).
			Render(gapContent)
		tabRow = lipgloss.JoinHorizontal(lipgloss.Bottom, tabRow, gap)
	}

	return tabRow
}

func (m model) renderDocsTabContent() string {
	if len(m.docsPages) == 0 {
		return ""
	}
	if m.docsActiveTab >= len(m.docsPages) {
		return ""
	}
	return m.docsPages[m.docsActiveTab].Content
}

func switchDocsFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	handledKey := false

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, defaultKeyMap.Help):
			m.toggleHelp()
			handledKey = true

		case key.Matches(msg, defaultKeyMap.Back), key.Matches(msg, defaultKeyMap.ViewDocs):
			m.clearDocsView()
			m.table.Focus()
			return m, nil

		case key.Matches(msg, defaultKeyMap.Up):
			m.docsViewer, _ = m.docsViewer.Update(msg)
			return m, nil

		case key.Matches(msg, defaultKeyMap.Down):
			m.docsViewer, _ = m.docsViewer.Update(msg)
			return m, nil

		case key.Matches(msg, defaultKeyMap.TabNext):
			m.docsActiveTab = docsNextTab(m.docsActiveTab, len(m.docsPages))
			m.docsViewer.GotoTop()
			return m, func() tea.Msg { return renderDocsMsg("tab switch") }

		case key.Matches(msg, defaultKeyMap.TabPrev):
			m.docsActiveTab = docsPrevTab(m.docsActiveTab, len(m.docsPages))
			m.docsViewer.GotoTop()
			return m, func() tea.Msg { return renderDocsMsg("tab switch") }
		}
	}

	if !handledKey {
		m.docsViewer, cmd = m.docsViewer.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}
