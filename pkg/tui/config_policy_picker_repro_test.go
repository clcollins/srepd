package tui

// Deterministic regression harness for the OB-4 policy picker bug: the
// policy Select's async OptionsFunc fetch completed and was accepted by
// huh, but the picker appeared to show only the skip/manual escapes.
// Root cause: huh's Select.selectOption() moves the cursor to the option
// matching the current bound value and sets viewport.YOffset to it
// unclamped; the bound value started as "" which was also the skip
// sentinel's value, so the cursor (and viewport) jumped to Skip, hiding
// every policy above it. The fix gives the skip sentinel a non-empty
// value so the initial empty choice matches no option and the cursor
// stays on index 0 — the first recommended policy (see
// buildPolicyOptions).
//
// The pump emulates the bubbletea runtime: Update() is called with each
// delivered message, returned commands are executed immediately (like
// goroutine start), and the resulting messages are queued FIFO — except
// huh's async updateOptionsMsg results, which are held back and released
// only on request, simulating a slow API fetch relative to other traffic.

import (
	"reflect"
	"strings"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	pkgconfig "github.com/clcollins/srepd/pkg/config"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const maxTicksPerPhase = 4

type wizardPump struct {
	t      *testing.T
	m      tea.Model
	queue  []tea.Msg
	held   []tea.Msg
	ticks  int
	blinks int
}

// execCmd runs a command tree, flattening tea.Batch and tea.Sequence
// (the latter via reflection — sequenceMsg is unexported) into the
// ordered list of messages it produces.
func execCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, execCmd(c)...)
		}
		return out
	}
	rv := reflect.ValueOf(msg)
	cmdType := reflect.TypeOf((*tea.Cmd)(nil)).Elem()
	if rv.Kind() == reflect.Slice && rv.Type().Elem() == cmdType {
		// tea.sequenceMsg
		var out []tea.Msg
		for i := 0; i < rv.Len(); i++ {
			if c, ok := rv.Index(i).Interface().(tea.Cmd); ok {
				out = append(out, execCmd(c)...)
			}
		}
		return out
	}
	return []tea.Msg{msg}
}

func isAsyncOptionsMsg(msg tea.Msg) bool {
	return strings.Contains(reflect.TypeOf(msg).String(), "updateOptionsMsg")
}

func (p *wizardPump) collect(cmd tea.Cmd) {
	for _, msg := range execCmd(cmd) {
		switch msg.(type) {
		case tea.WindowSizeMsg:
			// tea.WindowSize() queries the real terminal; tests inject
			// their own size, so drop command-produced ones.
			continue
		}
		if isAsyncOptionsMsg(msg) {
			p.t.Logf("pump: holding async %s", reflect.TypeOf(msg))
			p.held = append(p.held, msg)
			continue
		}
		p.queue = append(p.queue, msg)
	}
}

func (p *wizardPump) drain() {
	for i := 0; i < 200 && len(p.queue) > 0; i++ {
		msg := p.queue[0]
		p.queue = p.queue[1:]
		if _, ok := msg.(spinner.TickMsg); ok {
			p.ticks++
			if p.ticks > maxTicksPerPhase {
				continue
			}
		}
		if _, ok := msg.(cursor.BlinkMsg); ok {
			p.blinks++
			if p.blinks > 2 {
				continue
			}
		}
		p.t.Logf("pump: deliver %s", reflect.TypeOf(msg))
		var cmd tea.Cmd
		p.m, cmd = p.m.Update(msg)
		p.collect(cmd)
	}
}

func (p *wizardPump) send(msg tea.Msg) {
	p.queue = append(p.queue, msg)
	p.drain()
}

// releaseHeld delivers the held async fetch results, simulating the
// slow API call finally returning after other traffic has settled.
func (p *wizardPump) releaseHeld() {
	held := p.held
	p.held = nil
	p.ticks = 0
	p.blinks = 0
	for _, msg := range held {
		p.t.Logf("pump: releasing async %s", reflect.TypeOf(msg))
		p.queue = append(p.queue, msg)
	}
	p.drain()
}

func (p *wizardPump) view() string {
	return p.m.(model).View()
}

// logState dumps the shared wizard state so the trace shows exactly what
// the async OptionsFunc closures could observe at each step.
func (p *wizardPump) logState(label string) {
	mm := p.m.(model)
	p.t.Logf("state[%s]: SelectedTeams=%v TokenInput=%q SilentPolicyChoice=%q",
		label, mm.configState.SelectedTeams, mm.configState.TokenInput, mm.configState.SilentPolicyChoice)
}

func keyEnter() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }

// enterUntil presses enter (releasing held async fetches between presses)
// until the rendered view contains substr, failing after max presses. It
// lets walk-the-wizard tests tolerate environment-dependent groups (e.g.
// the agent confirm only appears when the claude CLI is detected).
func (p *wizardPump) enterUntil(t *testing.T, substr string, max int) {
	t.Helper()
	for i := 0; i < max; i++ {
		if strings.Contains(p.view(), substr) {
			return
		}
		p.send(keyEnter())
		p.releaseHeld()
	}
	t.Fatalf("never reached %q after %d enters; last view:\n%s", substr, max, p.view())
}

func keyRunes(s string) tea.Msg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func policyPickerMockClient() *pd.MockPagerDutyClient {
	// Enough policies that the pre-fix 4-line viewport could not show them
	// all: 1 silent-named + 5 other silent candidates + 2 real.
	policies := []pagerduty.EscalationPolicy{
		{APIObject: pagerduty.APIObject{ID: "POL_NULL_1"}, Name: "Null Route Alpha"},
		{APIObject: pagerduty.APIObject{ID: "POL_NULL_2"}, Name: "Null Route Bravo"},
		{APIObject: pagerduty.APIObject{ID: "POL_NULL_3"}, Name: "Null Route Charlie"},
		{APIObject: pagerduty.APIObject{ID: "POL_NULL_4"}, Name: "Null Route Delta"},
		{APIObject: pagerduty.APIObject{ID: "POL_NULL_5"}, Name: "Null Route Echo"},
		{APIObject: pagerduty.APIObject{ID: "POL_SILENT"}, Name: "CAD Silent Test"},
		{
			APIObject: pagerduty.APIObject{ID: "POL_HUMAN"},
			Name:      "Humans On Call",
			EscalationRules: []pagerduty.EscalationRule{
				{Targets: []pagerduty.APIObject{{Type: "schedule_reference", ID: "SCHED_1"}}},
			},
		},
		{
			APIObject: pagerduty.APIObject{ID: "POL_HUMAN_2"},
			Name:      "Secondary On-Call",
			EscalationRules: []pagerduty.EscalationRule{
				{Targets: []pagerduty.APIObject{{Type: "schedule_reference", ID: "SCHED_2"}}},
			},
		},
	}
	return &pd.MockPagerDutyClient{
		ListEscalationPoliciesResponses: []pagerduty.ListEscalationPoliciesResponse{
			{EscalationPolicies: policies},
		},
	}
}

// TestConfigWizardPolicyPickerRendersFetchedPolicies drives the real
// wizard form end-to-end (token → teams → policy) with async fetches
// arriving late, and requires the fetched policies to appear in the
// policy picker.
func TestConfigWizardPolicyPickerRendersFetchedPolicies(t *testing.T) {
	m := createConfigTestModel()
	mock := policyPickerMockClient()
	m.pdClientFactory = func(_ string) pd.PagerDutyClient { return mock }

	p := &wizardPump{t: t, m: m}

	p.send(tea.WindowSizeMsg{Width: 120, Height: 50})
	p.send(newConfigWizardReadyMsg(pkgconfig.ExistingConfig{}, pkgconfig.KeepDefaults{}, true))

	// Token step: type a token and advance. Validation hits the mock.
	p.send(keyRunes("mock-token"))
	p.send(keyEnter())
	p.releaseHeld() // teams fetch returns

	view := p.view()
	require.Contains(t, view, "Mock Team Alpha", "teams should render after the async fetch lands")

	// Teams step: toggle the first team and advance to the policy picker.
	p.send(keyRunes("x"))
	p.logState("after toggle")
	p.send(keyEnter())
	p.logState("after enter to policy group")

	// The fetch is in flight (held); huh's native loading indicator must
	// render. It only appears once a message arrives >25ms after dispatch,
	// which is why srepd must forward spinner ticks to the form.
	assert.Contains(t, p.view(), "Loading",
		"picker must show a loading indicator while the fetch is in flight")

	p.releaseHeld() // policy fetch returns
	p.logState("after policy fetch release")

	t.Logf("mock call counts: %v", mock.CallCounts)

	view = p.view()
	assert.Contains(t, view, "CAD Silent Test",
		"fetched policies must render in the picker after the async fetch lands")
	assert.NotContains(t, view, "Loading",
		"loading indicator must be replaced by the fetched list")
	assert.Contains(t, view, "Skip — configure later",
		"static escape options should always be present")
	assert.Less(t, strings.Index(view, "CAD Silent Test"), strings.Index(view, "Null Route Alpha"),
		`silent-named candidates must render above the other candidates`)
	assert.Less(t, strings.Index(view, "Null Route Echo"), strings.Index(view, "Skip — configure later"),
		"candidates must render above the escape options")
	assert.Contains(t, view, "Secondary On-Call",
		"the picker must use the available window height — with 8 policies the pre-fix 4-line viewport would hide the later entries")
	assert.Equal(t, "POL_SILENT", p.m.(model).configState.SilentPolicyChoice,
		"cursor must start on the first recommended policy (index 0)")

	// Arrow keys must move the cursor through the list, not scroll the
	// list under a top-pinned cursor. With an explicit field height, huh
	// v1.0.0 re-pins viewport.YOffset to the selected index on every
	// update, so two downs would scroll the first option out of view.
	p.send(tea.KeyMsg{Type: tea.KeyDown})
	p.send(tea.KeyMsg{Type: tea.KeyDown})
	view = p.view()
	assert.Equal(t, "POL_NULL_2", p.m.(model).configState.SilentPolicyChoice,
		"two downs must land on the third option")
	assert.Contains(t, view, "CAD Silent Test",
		"moving the cursor down must not scroll the top of the list out of view")
}

// Regression: three identical "Configure advanced options?" confirm groups
// shipped on main (one duplicate re-added per merge in PRs #370 and #371),
// forcing users to answer the same prompt three times. Exactly one enter
// must dismiss it.
func TestConfigWizardAdvancedOptionsConfirmAdvancesInOneEnter(t *testing.T) {
	m := createConfigTestModel()
	mock := policyPickerMockClient()
	m.pdClientFactory = func(_ string) pd.PagerDutyClient { return mock }

	p := &wizardPump{t: t, m: m}
	p.send(tea.WindowSizeMsg{Width: 120, Height: 50})
	p.send(newConfigWizardReadyMsg(pkgconfig.ExistingConfig{}, pkgconfig.KeepDefaults{}, true))

	// Token and teams steps, as in the main walk.
	p.send(keyRunes("mock-token"))
	p.send(keyEnter())
	p.releaseHeld() // teams fetch returns
	p.send(keyRunes("x"))
	p.send(keyEnter())
	p.releaseHeld() // policy fetch returns

	p.enterUntil(t, "Configure advanced options?", 12)
	p.send(keyEnter()) // answer the confirm (default: No)
	p.releaseHeld()

	assert.NotContains(t, p.view(), "Configure advanced options?",
		"one enter must dismiss the advanced-options confirm — the group must exist exactly once")
}
