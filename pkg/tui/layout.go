package tui

import (
	"math"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	layoutHeaderLines       = 2
	layoutFooterLines       = 1
	layoutFooterNewline     = 1
	layoutInputLines        = 1
	layoutBottomStatusLines = 1

	tableFixedOverhead = layoutHeaderLines + layoutFooterLines + layoutFooterNewline + layoutInputLines + layoutBottomStatusLines

	incidentViewerTabBarLines   = 3
	incidentViewerTabBarNewline = 1
	incidentViewerFixedOverhead = layoutHeaderLines + incidentViewerTabBarLines + incidentViewerTabBarNewline + layoutBottomStatusLines

	configFormBottomPadding = 1
	configFormReserved      = layoutHeaderLines + layoutBottomStatusLines + configFormBottomPadding

	layoutWatcherPaneHeight     = 5
	layoutWatcherBorderOverhead = 2
	layoutWatcherHeaderLines    = 1
	layoutMinWatcherHeight      = 3

	layoutMinTableHeight          = 4
	layoutMinIncidentViewerHeight = 10
	layoutMinFormHeight           = 1
	layoutNumTableColumns         = 4

	layoutMaxClusterIDWidth = 40
	layoutMaxServiceWidth   = 50
)

type Layout struct {
	WindowWidth  int
	WindowHeight int

	ContentWidth int
	HelpLines    int
	HelpWidth    int

	TableWidth  int
	TableHeight int
	ColumnWidth int

	IncidentViewerWidth  int
	IncidentViewerHeight int

	WatcherWidth  int
	WatcherHeight int

	FormWidth            int
	FormHeight           int
	TeamSelectFormHeight int

	ClusterSelectClusterIDWidth int
	ClusterSelectServiceWidth   int
}

func computeLayout(ws tea.WindowSizeMsg, styles Styles, helpView string, watcherLines int) Layout {
	mainHOverhead := styles.Main.GetHorizontalMargins() +
		styles.Main.GetHorizontalPadding() +
		styles.Main.GetHorizontalBorderSize()
	mainVOverhead := styles.Main.GetVerticalMargins() +
		styles.Main.GetVerticalPadding() +
		styles.Main.GetVerticalBorderSize()

	containerVOverhead := styles.TableContainer.GetVerticalMargins() +
		styles.TableContainer.GetVerticalPadding() +
		styles.TableContainer.GetVerticalBorderSize()
	containerHOverhead := styles.TableContainer.GetHorizontalMargins() +
		styles.TableContainer.GetHorizontalPadding() +
		styles.TableContainer.GetHorizontalBorderSize()

	cellStyle := styles.Table.Cell
	cellHOverhead := (cellStyle.GetHorizontalPadding() +
		cellStyle.GetHorizontalMargins() +
		cellStyle.GetHorizontalBorderSize()) * layoutNumTableColumns

	contentWidth := ws.Width - mainHOverhead
	helpWidth := contentWidth
	helpLines := strings.Count(helpView, "\n") + 1

	tableWidth := ws.Width - mainHOverhead - containerHOverhead - cellHOverhead
	tableHeight := ws.Height - mainVOverhead - containerVOverhead - tableFixedOverhead - helpLines - watcherLines
	if tableHeight < layoutMinTableHeight {
		tableHeight = layoutMinTableHeight
	}

	watcherHeight := layoutWatcherPaneHeight
	if watcherHeight < layoutMinWatcherHeight {
		watcherHeight = layoutMinWatcherHeight
	}
	watcherWidth := ws.Width - mainHOverhead - containerHOverhead

	columnWidth := int(math.Ceil(float64(tableWidth-idWidth-dotWidth) / float64(2)))

	tabWindowBorders := styles.TabWindow.GetHorizontalBorderSize()
	incidentViewerWidth := ws.Width - mainHOverhead - tabWindowBorders
	incidentViewerHeight := ws.Height - incidentViewerFixedOverhead
	if incidentViewerHeight < layoutMinIncidentViewerHeight {
		incidentViewerHeight = layoutMinIncidentViewerHeight
	}

	formWidth := ws.Width
	formHeight := ws.Height - configFormReserved
	if formHeight < layoutMinFormHeight {
		formHeight = layoutMinFormHeight
	}
	teamSelectFormHeight := ws.Height

	clusterIDWidth := contentWidth * 2 / 5
	if clusterIDWidth > layoutMaxClusterIDWidth {
		clusterIDWidth = layoutMaxClusterIDWidth
	}
	if clusterIDWidth < 10 {
		clusterIDWidth = 10
	}
	serviceWidth := contentWidth * 3 / 5
	if serviceWidth > layoutMaxServiceWidth {
		serviceWidth = layoutMaxServiceWidth
	}
	if serviceWidth < 10 {
		serviceWidth = 10
	}

	return Layout{
		WindowWidth:                 ws.Width,
		WindowHeight:                ws.Height,
		ContentWidth:                contentWidth,
		HelpLines:                   helpLines,
		HelpWidth:                   helpWidth,
		TableWidth:                  tableWidth,
		TableHeight:                 tableHeight,
		ColumnWidth:                 columnWidth,
		IncidentViewerWidth:         incidentViewerWidth,
		IncidentViewerHeight:        incidentViewerHeight,
		FormWidth:                   formWidth,
		FormHeight:                  formHeight,
		TeamSelectFormHeight:        teamSelectFormHeight,
		WatcherWidth:                watcherWidth,
		WatcherHeight:               watcherHeight,
		ClusterSelectClusterIDWidth: clusterIDWidth,
		ClusterSelectServiceWidth:   serviceWidth,
	}
}

func (m *model) recomputeLayout() {
	mainHOverhead := m.styles.Main.GetHorizontalMargins() +
		m.styles.Main.GetHorizontalPadding() +
		m.styles.Main.GetHorizontalBorderSize()
	m.help.Width = windowSize.Width - mainHOverhead

	var helpKeyMap help.KeyMap
	if m.chordHelpActive {
		helpKeyMap = chordKeymap{prefix: m.chordPrefix}
	} else if m.input.Focused() {
		helpKeyMap = inputModeKeyMap
	} else {
		helpKeyMap = defaultKeyMap
	}

	helpView := m.help.View(helpKeyMap)

	watcherLines := 0
	if m.watcherExpanded {
		watcherLines = layoutWatcherPaneHeight + layoutWatcherBorderOverhead + layoutWatcherHeaderLines
	}

	m.layout = computeLayout(windowSize, m.styles, helpView, watcherLines)
	m.table.SetHeight(m.layout.TableHeight)

	if m.watcherExpanded {
		m.watcherViewport.Width = m.layout.WatcherWidth
		m.watcherViewport.Height = m.layout.WatcherHeight
	}
}
