package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/clcollins/srepd/pkg/pd"
)

const (
	defaultTitle = "PagerDuty Incidents"
)

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	statusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}).
				Render
)

type statusMsg int

type model struct {
	context  context.Context
	config   *pd.Config
	spinner  spinner.Model
	list     list.Model
	errorLog []error
	startup  bool
}

func InitialModel(ctx context.Context, c *pd.Config) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	incidentList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	incidentList.Title = defaultTitle
	return model{context: ctx, config: c, list: incidentList, spinner: s, startup: true}
}

func (m *model) UpdateList(ctx context.Context, c *pd.Config) tea.Msg {
	pdIncidents, err := pd.GetIncidents(ctx, c)
	if err != nil {
		return err
	}

	items := make([]list.Item, len(pdIncidents))
	for i, p := range pdIncidents {
		items[i] = incident{title: p.Title, id: p.ID, status: p.Status, summary: p.Summary, raw: p}
	}

	m.list.SetItems(items)
	return statusMsg(0)
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.startup {
		m.UpdateList(m.context, m.config)
		m.startup = false
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := appStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, defaultKeyMap.Quit):
			return m, tea.Quit
		case key.Matches(msg, defaultKeyMap.Refresh):
			m.UpdateList(m.context, m.config)
			return m, nil
		}
	case statusMsg:
		return m, nil
	}
	return m, nil
}

func (m model) View() string {
	return appStyle.Render(m.list.View())
}
