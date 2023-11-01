package tui

import (
	"github.com/PagerDuty/go-pagerduty"
)

type incident struct {
	id      string
	title   string
	status  string
	summary string
	raw     pagerduty.Incident
}

func (i incident) Title() string       { return i.title }
func (i incident) Summary() string     { return i.summary }
func (i incident) FilterValue() string { return i.title }
