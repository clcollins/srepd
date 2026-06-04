package tui

import (
	"regexp"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

var hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{3}([0-9a-fA-F]{3})?$`)

func isValidHexColor(s string) bool {
	return hexColorPattern.MatchString(s)
}

type Theme struct {
	Text       lipgloss.AdaptiveColor
	Background lipgloss.AdaptiveColor
	Border     lipgloss.AdaptiveColor
	Highlight  lipgloss.AdaptiveColor
	Selected   lipgloss.AdaptiveColor
	Warning    lipgloss.AdaptiveColor
	Error      lipgloss.AdaptiveColor
	Muted      lipgloss.AdaptiveColor
	Tab        lipgloss.AdaptiveColor
}

type Styles struct {
	Main           lipgloss.Style
	Padded         lipgloss.Style
	Muted          lipgloss.Style
	Warning        lipgloss.Style
	Error          lipgloss.Style
	TableContainer lipgloss.Style
	Table          table.Styles
	ActiveTab      lipgloss.Style
	InactiveTab    lipgloss.Style
	TabWindow      lipgloss.Style
}

func DefaultTheme() Theme {
	return Theme{
		Text:       lipgloss.AdaptiveColor{Dark: "#778da9", Light: "#778da9"},
		Background: lipgloss.AdaptiveColor{},
		Border:     lipgloss.AdaptiveColor{Dark: "#415a77", Light: "#415a77"},
		Highlight:  lipgloss.AdaptiveColor{Dark: "#ffffff", Light: "#ffffff"},
		Selected:   lipgloss.AdaptiveColor{Dark: "#415a77", Light: "#415a77"},
		Warning:    lipgloss.AdaptiveColor{Dark: "#a4133c", Light: "#a4133c"},
		Error:      lipgloss.AdaptiveColor{Dark: "#0d1b2a", Light: "#0d1b2a"},
		Muted:      lipgloss.AdaptiveColor{Dark: "#5C5C5C", Light: "#9B9B9B"},
		Tab:        lipgloss.AdaptiveColor{Dark: "#7D56F4", Light: "#874BFD"},
	}
}

func ThemeFromConfig(colors map[string]string) Theme {
	theme := DefaultTheme()
	if colors == nil {
		return theme
	}

	overrides := map[string]*lipgloss.AdaptiveColor{
		"text":      &theme.Text,
		"border":    &theme.Border,
		"highlight": &theme.Highlight,
		"selected":  &theme.Selected,
		"warning":   &theme.Warning,
		"error":     &theme.Error,
		"muted":     &theme.Muted,
		"tab":       &theme.Tab,
	}

	for key, target := range overrides {
		if val, ok := colors[key]; ok && isValidHexColor(val) {
			*target = lipgloss.AdaptiveColor{Dark: val, Light: val}
		}
	}

	return theme
}

func BuildStyles(theme Theme) Styles {
	main := lipgloss.NewStyle().
		Margin(0, 0).
		Padding(0, 0).
		Foreground(theme.Text).
		Background(theme.Background).
		BorderForeground(theme.Border).
		BorderBackground(theme.Background)

	padded := main.Padding(0, 2, 0, 1)

	inactiveTabBorder := tabBorderWithBottom("┴", "─", "┴")
	activeTabBorder := tabBorderWithBottom("┘", " ", "└")

	inactiveTab := lipgloss.NewStyle().
		Border(inactiveTabBorder, true).
		BorderForeground(theme.Tab).
		Padding(0, 1)

	activeTab := inactiveTab.Border(activeTabBorder, true)

	tabWindow := lipgloss.NewStyle().
		BorderForeground(theme.Tab).
		Border(lipgloss.NormalBorder()).
		UnsetBorderTop()

	tableContainer := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true).
		BorderForeground(theme.Border)

	tableCell := lipgloss.NewStyle().Padding(0, 1)
	tableHeader := lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.RoundedBorder(), false, false, true).
		Foreground(theme.Highlight).
		Background(theme.Background)
	tableSelected := lipgloss.NewStyle().
		Border(lipgloss.HiddenBorder(), false).
		Background(theme.Selected).
		Foreground(theme.Highlight).
		Bold(true)

	return Styles{
		Main:    main,
		Padded:  padded,
		Muted:   lipgloss.NewStyle().Foreground(theme.Muted),
		Warning: lipgloss.NewStyle().Foreground(theme.Highlight).Background(theme.Warning),
		Error: lipgloss.NewStyle().
			Bold(true).
			Width(64).
			Border(lipgloss.RoundedBorder()).
			Foreground(theme.Highlight).
			Background(theme.Error).
			BorderForeground(theme.Border).
			Padding(1, 3, 1, 3),
		TableContainer: tableContainer,
		Table: table.Styles{
			Cell:     tableCell,
			Selected: tableSelected,
			Header:   tableHeader,
		},
		ActiveTab:   activeTab,
		InactiveTab: inactiveTab,
		TabWindow:   tabWindow,
	}
}
