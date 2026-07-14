package tui

// Preset safety gate (supply-chain hardening): a --preset is remote input,
// and terminal, editor, and cluster_login_command values are commands srepd
// executes. When a preset seeded any of them, saving requires two extra
// explicit confirmations after "Save changes?" — a bold-red review of the
// commands, then vouching for the source. Declining either discards the
// save. Wizard-typed or existing-config values never see the gate.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	pkgconfig "github.com/clcollins/srepd/pkg/config"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fakePresetTerminal = "FAKE-terminal --run"
	fakePresetLogin    = "FAKE-login %%CLUSTER_ID%%"
	fakePresetSource   = "https://preset.example.test/FAKE-team.yaml"
)

func presetWizardReadyMsg(applied pkgconfig.PresetApplied) configWizardReadyMsg {
	existing := pkgconfig.ExistingConfig{}
	if applied.Terminal {
		existing.Terminal = fakePresetTerminal
	}
	if applied.ClusterLogin {
		existing.ClusterLoginCommand = fakePresetLogin
	}
	return configWizardReadyMsg{
		existing:      existing,
		kd:            pkgconfig.KeepDefaults{},
		isNewFile:     true,
		teamNames:     map[string]string{},
		policyNames:   map[string]string{},
		presetApplied: applied,
	}
}

// walkToSaveConfirm drives the wizard from launch to the "Save changes?"
// confirm using the pump.
func walkToSaveConfirm(t *testing.T, p *wizardPump) {
	t.Helper()
	p.send(keyEnter()) // welcome step
	p.send(keyRunes("mock-token"))
	p.send(keyEnter())
	p.releaseHeld() // teams fetch returns
	p.send(keyRunes("x"))
	p.send(keyEnter())
	p.releaseHeld() // policy fetch returns
	p.enterUntil(t, "Save changes?", 14)
}

func newPresetSafetyPump(t *testing.T, applied pkgconfig.PresetApplied) (*wizardPump, *tuiMockFS) {
	t.Helper()
	m := createConfigTestModel()
	mfs := &tuiMockFS{}
	m.configFS = mfs
	mock := policyPickerMockClient()
	m.pdClientFactory = func(_ string) pd.PagerDutyClient { return mock }

	p := &wizardPump{t: t, m: m}
	p.send(tea.WindowSizeMsg{Width: 120, Height: 50})
	p.send(presetWizardReadyMsg(applied))
	walkToSaveConfirm(t, p)
	return p, mfs
}

func TestPresetSafety_GateShownAndSavesAfterBothConfirms(t *testing.T) {
	applied := pkgconfig.PresetApplied{
		Terminal:     true,
		ClusterLogin: true,
		Source:       fakePresetSource,
	}
	p, mfs := newPresetSafetyPump(t, applied)

	// Save changes? defaults to Yes; enter must land on the red warning,
	// not complete the form.
	p.send(keyEnter())
	view := p.view()
	require.Contains(t, view, "SECURITY",
		"preset-seeded executable fields must trigger the safety warning after Save")
	assert.Contains(t, view, fakePresetTerminal, "the warning must list the terminal command")
	assert.Contains(t, view, fakePresetLogin, "the warning must list the cluster login command")
	assert.True(t, p.m.(model).configMode, "form must not complete before the gate")

	// Affirm the commands are safe → the trust-the-source confirm.
	p.send(keyRunes("y"))
	view = p.view()
	require.Contains(t, view, "trust the source",
		"affirming the commands must lead to the source-trust confirm")
	assert.Contains(t, view, fakePresetSource, "the trust confirm must show the preset source")

	// Affirm trust → form completes and the config is written.
	p.send(keyRunes("y"))
	p.releaseHeld()
	assert.False(t, p.m.(model).configMode, "form must complete after both confirmations")
	assert.Contains(t, string(mfs.writeData), fakePresetTerminal,
		"config must be written with the confirmed values")
}

func TestPresetSafety_DecliningCommandsDiscardsSave(t *testing.T) {
	applied := pkgconfig.PresetApplied{Terminal: true, Source: fakePresetSource}
	p, mfs := newPresetSafetyPump(t, applied)

	p.send(keyEnter()) // Save changes? → warning
	require.Contains(t, p.view(), "SECURITY")

	p.send(keyRunes("n")) // not safe → no trust prompt, form completes, discarded
	p.releaseHeld()
	assert.False(t, p.m.(model).configMode, "form must complete after declining")
	assert.Empty(t, mfs.writeData, "declining the command review must discard the save")
	// The "preset commands not confirmed" status is set on completion but
	// immediately superseded by the follow-up incident-list refresh in the
	// synchronous pump, so it is not asserted here — the empty write is the
	// behavior under test.
}

func TestPresetSafety_NoGateWithoutPreset(t *testing.T) {
	p, mfs := newPresetSafetyPump(t, pkgconfig.PresetApplied{})

	assert.NotContains(t, p.view(), "SECURITY",
		"no preset means no safety warning anywhere in the flow")
	p.send(keyEnter()) // Save changes? → completes directly
	p.releaseHeld()
	assert.False(t, p.m.(model).configMode, "form must complete without extra gates")
	assert.NotEmpty(t, mfs.writeData, "config must be written")
}

func TestPresetSafety_NoGateForNonExecutableFields(t *testing.T) {
	// Teams/policies are only sent to the PagerDuty API, never executed —
	// they must not trigger the gate.
	applied := pkgconfig.PresetApplied{Teams: true, Silent: true, Custom: true, Source: fakePresetSource}
	p, mfs := newPresetSafetyPump(t, applied)

	p.send(keyEnter())
	p.releaseHeld()
	assert.False(t, p.m.(model).configMode, "form must complete without extra gates")
	assert.NotEmpty(t, mfs.writeData, "config must be written")
}
