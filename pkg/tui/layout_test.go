package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/stretchr/testify/assert"
)

func defaultStyles() Styles {
	return BuildStyles(DefaultTheme())
}

func shortHelp() string {
	return "? help · esc back · q quit"
}

func longHelp() string {
	return strings.Repeat("line\n", 8) + "last line"
}

func TestComputeLayout_StandardTerminal(t *testing.T) {
	t.Run("80x24 produces usable dimensions", func(t *testing.T) {
		ws := tea.WindowSizeMsg{Width: 80, Height: 24}
		l := computeLayout(ws, defaultStyles(), shortHelp(), false)

		assert.Equal(t, 80, l.WindowWidth)
		assert.Equal(t, 24, l.WindowHeight)
		assert.Greater(t, l.ContentWidth, 50)
		assert.GreaterOrEqual(t, l.TableHeight, 10)
		assert.Greater(t, l.ColumnWidth, 0)
		assert.Greater(t, l.IncidentViewerHeight, 0)
		assert.Greater(t, l.FormHeight, 0)
	})
}

func TestComputeLayout_SmallTerminal(t *testing.T) {
	t.Run("80x10 floors at minimums", func(t *testing.T) {
		ws := tea.WindowSizeMsg{Width: 80, Height: 10}
		l := computeLayout(ws, defaultStyles(), shortHelp(), false)

		assert.Equal(t, layoutMinTableHeight, l.TableHeight)
		assert.Equal(t, layoutMinIncidentViewerHeight, l.IncidentViewerHeight)
	})
}

func TestComputeLayout_LargeTerminal(t *testing.T) {
	t.Run("200x60 scales proportionally", func(t *testing.T) {
		ws := tea.WindowSizeMsg{Width: 200, Height: 60}
		l := computeLayout(ws, defaultStyles(), shortHelp(), false)

		assert.Greater(t, l.TableHeight, 30)
		assert.Greater(t, l.ColumnWidth, 50)
		assert.Greater(t, l.IncidentViewerHeight, 40)
	})
}

func TestComputeLayout_HelpExpandedReducesHeight(t *testing.T) {
	t.Run("expanded help produces shorter table", func(t *testing.T) {
		ws := tea.WindowSizeMsg{Width: 80, Height: 24}
		styles := defaultStyles()

		compactLayout := computeLayout(ws, styles, shortHelp(), false)
		expandedLayout := computeLayout(ws, styles, longHelp(), false)

		assert.Greater(t, compactLayout.TableHeight, expandedLayout.TableHeight,
			"compact help should yield taller table than expanded help")
	})
}

func TestComputeLayout_TableAndIncidentViewerConsistent(t *testing.T) {
	t.Run("both use named constants and are positive", func(t *testing.T) {
		ws := tea.WindowSizeMsg{Width: 80, Height: 24}
		l := computeLayout(ws, defaultStyles(), shortHelp(), false)

		assert.Greater(t, l.TableHeight, 0)
		assert.Greater(t, l.IncidentViewerHeight, 0)
		assert.Equal(t, 6, tableFixedOverhead, "tableFixedOverhead should be 6")
		assert.Equal(t, 7, incidentViewerFixedOverhead, "incidentViewerFixedOverhead should be 7")
	})
}

func TestComputeLayout_FormHeight(t *testing.T) {
	t.Run("form heights computed from constants and container frame", func(t *testing.T) {
		ws := tea.WindowSizeMsg{Width: 80, Height: 24}
		styles := defaultStyles()
		l := computeLayout(ws, styles, shortHelp(), false)

		vFrame := styles.FormContainer.GetVerticalFrameSize()
		hFrame := styles.FormContainer.GetHorizontalFrameSize()
		assert.Equal(t, 24-configFormReserved-vFrame, l.FormHeight)
		assert.Equal(t, 24-vFrame, l.TeamSelectFormHeight)
		assert.Equal(t, 80-hFrame, l.FormWidth)
	})
}

func TestComputeLayout_ZeroHeight(t *testing.T) {
	t.Run("zero height floors at minimums", func(t *testing.T) {
		ws := tea.WindowSizeMsg{Width: 80, Height: 0}
		l := computeLayout(ws, defaultStyles(), shortHelp(), false)

		assert.Equal(t, layoutMinTableHeight, l.TableHeight)
		assert.Equal(t, layoutMinIncidentViewerHeight, l.IncidentViewerHeight)
		assert.GreaterOrEqual(t, l.FormHeight, layoutMinFormHeight)
	})
}

func TestComputeLayout_ClusterSelectWidthsCapped(t *testing.T) {
	t.Run("wide terminal caps cluster select widths", func(t *testing.T) {
		ws := tea.WindowSizeMsg{Width: 200, Height: 40}
		l := computeLayout(ws, defaultStyles(), shortHelp(), false)

		assert.LessOrEqual(t, l.ClusterSelectClusterIDWidth, layoutMaxClusterIDWidth)
		assert.LessOrEqual(t, l.ClusterSelectServiceWidth, layoutMaxServiceWidth)
	})

	t.Run("narrow terminal uses proportional widths", func(t *testing.T) {
		ws := tea.WindowSizeMsg{Width: 60, Height: 24}
		l := computeLayout(ws, defaultStyles(), shortHelp(), false)

		assert.Less(t, l.ClusterSelectClusterIDWidth, layoutMaxClusterIDWidth)
		assert.Less(t, l.ClusterSelectServiceWidth, layoutMaxServiceWidth)
		assert.Greater(t, l.ClusterSelectClusterIDWidth, 0)
		assert.Greater(t, l.ClusterSelectServiceWidth, 0)
	})
}

func TestComputeLayout_MatchesOldTableHeight(t *testing.T) {
	t.Run("regression: matches old recalculateTableHeight behavior", func(t *testing.T) {
		ws := tea.WindowSizeMsg{Width: 80, Height: 24}
		styles := defaultStyles()
		helpView := shortHelp()

		l := computeLayout(ws, styles, helpView, false)

		helpLines := strings.Count(helpView, "\n") + 1
		oldFixedLines := 6
		mainOverhead := styles.Main.GetVerticalMargins() +
			styles.Main.GetVerticalPadding() +
			styles.Main.GetVerticalBorderSize()
		containerOverhead := styles.TableContainer.GetVerticalMargins() +
			styles.TableContainer.GetVerticalPadding() +
			styles.TableContainer.GetVerticalBorderSize()
		oldTableHeight := ws.Height - mainOverhead - containerOverhead - oldFixedLines - helpLines
		if oldTableHeight < 4 {
			oldTableHeight = 4
		}

		assert.Equal(t, oldTableHeight, l.TableHeight,
			"Layout.TableHeight must match old recalculateTableHeight output")
	})
}

func TestRecomputeLayout_SetsTableHeight(t *testing.T) {
	t.Run("recomputeLayout sets positive table height", func(t *testing.T) {
		m := createTestModel()
		m.help = newHelp()
		windowSize = tea.WindowSizeMsg{Width: 80, Height: 24}

		m.recomputeLayout()

		assert.Greater(t, m.layout.TableHeight, 0,
			"layout.TableHeight should be positive")
		assert.Greater(t, m.table.Height(), 0,
			"table viewport height should be positive after SetHeight")
	})
}

func TestWindowSizeMsg_PopulatesLayout(t *testing.T) {
	t.Run("WindowSizeMsg populates layout on model", func(t *testing.T) {
		m := createTestModel()
		m.help = newHelp()

		sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 50}
		result, _ := m.windowSizeMsgHandler(sizeMsg)
		updated := result.(model)

		assert.Equal(t, 120, updated.layout.WindowWidth)
		assert.Equal(t, 50, updated.layout.WindowHeight)
		assert.Greater(t, updated.layout.TableHeight, 0)
		assert.Greater(t, updated.layout.ColumnWidth, 0)
		assert.Greater(t, updated.layout.IncidentViewerHeight, 0)
	})
}

func TestWindowResize_ConfigMode(t *testing.T) {
	t.Run("resize during config mode updates form dimensions", func(t *testing.T) {
		m := createConfigTestModel()
		m.configMode = true
		confirm := true
		m.configForm = huh.NewForm(
			huh.NewGroup(huh.NewConfirm().Title("test").Value(&confirm)),
		).WithWidth(80).WithHeight(20)
		m.configForm.Init()

		sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 50}
		result, _ := m.windowSizeMsgHandler(sizeMsg)
		updated := result.(model)

		// Width is capped at layoutMaxFormWidth on wide terminals and the
		// container frame is subtracted from the height.
		assert.Equal(t, layoutMaxFormWidth, updated.layout.FormWidth)
		vFrame := updated.styles.FormContainer.GetVerticalFrameSize()
		assert.Equal(t, 50-configFormReserved-vFrame, updated.layout.FormHeight)
	})
}

func TestWindowResize_TeamSelectMode(t *testing.T) {
	t.Run("resize during team select mode forwards to form", func(t *testing.T) {
		m := createConfigTestModel()
		m.teamSelectMode = true
		confirm := true
		m.teamSelectForm = huh.NewForm(
			huh.NewGroup(huh.NewConfirm().Title("team select").Value(&confirm)),
		).WithHeight(30)
		m.teamSelectForm.Init()

		sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 50}
		result, _ := m.windowSizeMsgHandler(sizeMsg)
		updated := result.(model)

		vFrame := updated.styles.FormContainer.GetVerticalFrameSize()
		assert.Equal(t, 50-vFrame, updated.layout.TeamSelectFormHeight)
	})
}

func TestWindowResize_IncidentViewMode(t *testing.T) {
	t.Run("resize during incident view updates viewer dimensions", func(t *testing.T) {
		m := createTestModel()
		m.help = newHelp()
		m.viewingIncident = true

		sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 50}
		result, _ := m.windowSizeMsgHandler(sizeMsg)
		updated := result.(model)

		assert.Equal(t, updated.layout.IncidentViewerWidth, updated.incidentViewer.Width)
		assert.Equal(t, updated.layout.IncidentViewerHeight, updated.incidentViewer.Height)
	})
}

func TestComputeLayout_HelpExpandedReducesIncidentViewerHeight(t *testing.T) {
	t.Run("expanded help produces shorter incident viewer", func(t *testing.T) {
		ws := tea.WindowSizeMsg{Width: 80, Height: 24}
		styles := defaultStyles()

		compactLayout := computeLayout(ws, styles, shortHelp(), false)
		expandedLayout := computeLayout(ws, styles, longHelp(), false)

		assert.Greater(t, compactLayout.IncidentViewerHeight, expandedLayout.IncidentViewerHeight,
			"compact help should yield taller incident viewer than expanded help")
	})
}

func TestRecomputeLayout_UpdatesIncidentViewer(t *testing.T) {
	t.Run("recomputeLayout updates incidentViewer and logViewer heights", func(t *testing.T) {
		m := createTestModel()
		m.help = newHelp()
		windowSize = tea.WindowSizeMsg{Width: 80, Height: 40}

		m.help.ShowAll = false
		m.recomputeLayout()
		collapsedHeight := m.incidentViewer.Height

		m.help.ShowAll = true
		m.recomputeLayout()
		expandedHeight := m.incidentViewer.Height

		assert.Greater(t, collapsedHeight, expandedHeight,
			"incident viewer should shrink when help is expanded")
		assert.Equal(t, m.incidentViewer.Height, m.logViewer.Height,
			"logViewer height should match incidentViewer height")
	})
}

func TestComputeLayout_WatcherReducesTableHeight(t *testing.T) {
	t.Run("watcher expanded reduces table height", func(t *testing.T) {
		ws := tea.WindowSizeMsg{Width: 80, Height: 60}
		styles := defaultStyles()
		help := shortHelp()

		collapsed := computeLayout(ws, styles, help, false)
		expanded := computeLayout(ws, styles, help, true)

		assert.Greater(t, collapsed.TableHeight, expanded.TableHeight,
			"table should be shorter when watcher is expanded")
	})

	t.Run("watcher gets height when expanded", func(t *testing.T) {
		ws := tea.WindowSizeMsg{Width: 80, Height: 60}
		l := computeLayout(ws, defaultStyles(), shortHelp(), true)

		assert.GreaterOrEqual(t, l.WatcherHeight, layoutMinWatcherRows)
		assert.Greater(t, l.WatcherWidth, 0)
	})

	t.Run("table always gets at least minimum rows", func(t *testing.T) {
		ws := tea.WindowSizeMsg{Width: 80, Height: 60}
		l := computeLayout(ws, defaultStyles(), shortHelp(), true)

		assert.GreaterOrEqual(t, l.TableHeight, layoutMinTableRows)
	})

	t.Run("large terminal splits 2/3 table 1/3 watcher", func(t *testing.T) {
		ws := tea.WindowSizeMsg{Width: 80, Height: 80}
		l := computeLayout(ws, defaultStyles(), shortHelp(), true)

		assert.Greater(t, l.TableHeight, l.WatcherHeight,
			"table should be larger than watcher in 2/3 split")
	})

	t.Run("collapsed watcher gives zero height", func(t *testing.T) {
		ws := tea.WindowSizeMsg{Width: 80, Height: 60}
		l := computeLayout(ws, defaultStyles(), shortHelp(), false)

		assert.Equal(t, 0, l.WatcherHeight)
	})
}

func TestComputeLayout_WatcherWidthAccountsForPadding(t *testing.T) {
	styles := defaultStyles()
	watcherHOverhead := styles.WatcherContainer.GetHorizontalMargins() +
		styles.WatcherContainer.GetHorizontalPadding() +
		styles.WatcherContainer.GetHorizontalBorderSize()
	mainHOverhead := styles.Main.GetHorizontalMargins() +
		styles.Main.GetHorizontalPadding() +
		styles.Main.GetHorizontalBorderSize()

	widths := []int{60, 80, 120, 200}
	for _, w := range widths {
		ws := tea.WindowSizeMsg{Width: w, Height: 40}
		l := computeLayout(ws, styles, shortHelp(), true)

		expected := w - mainHOverhead - watcherHOverhead
		assert.Equal(t, expected, l.WatcherWidth,
			"width %d: watcher width must account for WatcherContainer overhead", w)
		assert.LessOrEqual(t, l.WatcherWidth+watcherHOverhead+mainHOverhead, w,
			"width %d: watcher + chrome must fit within terminal", w)
	}
}

func TestRecomputeLayout_WatcherExpanded(t *testing.T) {
	t.Run("watcher expanded reduces table height", func(t *testing.T) {
		m := createTestModel()
		m.help = newHelp()
		windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}

		m.watcherExpanded = false
		m.recomputeLayout()
		collapsedHeight := m.layout.TableHeight

		m.watcherExpanded = true
		m.recomputeLayout()
		expandedHeight := m.layout.TableHeight

		assert.Greater(t, collapsedHeight, expandedHeight)
		assert.GreaterOrEqual(t, expandedHeight, layoutMinTableRows)
	})
}
