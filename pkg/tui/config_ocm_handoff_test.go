package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/clcollins/srepd/pkg/ocm"
	"github.com/stretchr/testify/assert"
)

// OB-6 follow-through: OCM auth is skipped while the config wizard runs, but
// the session that continues after the wizard must get OCM enrichment exactly
// like a normal launch — otherwise first-run users see a degraded srepd and
// never come back.

func TestConnectOCMCmdIfNeeded_ReturnsCmdWhenNoClient(t *testing.T) {
	mock := createMockOCMClient()
	m := createConfigTestModel()
	m.ocmClient = nil
	m.ocmConnect = func() (ocm.OCMClient, error) { return mock, nil }

	cmd := m.connectOCMCmdIfNeeded()

	assert.NotNil(t, cmd, "should return a connect command when no OCM client exists")
	msg := cmd()
	ready, ok := msg.(OCMClientReadyMsg)
	assert.True(t, ok, "command must produce OCMClientReadyMsg, got %T", msg)
	assert.NoError(t, ready.Err)
	assert.Equal(t, ocm.OCMClient(mock), ready.Client)
}

func TestConnectOCMCmdIfNeeded_NilWhenClientPresent(t *testing.T) {
	m := createConfigTestModel()
	m.ocmClient = createMockOCMClient()

	assert.Nil(t, m.connectOCMCmdIfNeeded(), "must not reconnect when OCM client already exists")
}

func TestConnectOCMCmdIfNeeded_NilInDevMode(t *testing.T) {
	m := createConfigTestModel()
	m.ocmClient = nil
	m.devMode = true

	assert.Nil(t, m.connectOCMCmdIfNeeded(), "dev mode must not attempt OCM connections")
}

func TestConnectOCMCmdIfNeeded_ErrorProducesErrMsg(t *testing.T) {
	m := createConfigTestModel()
	m.ocmClient = nil
	m.ocmConnect = func() (ocm.OCMClient, error) { return nil, fmt.Errorf("auth cancelled") }

	cmd := m.connectOCMCmdIfNeeded()
	assert.NotNil(t, cmd)

	msg := cmd()
	ready, ok := msg.(OCMClientReadyMsg)
	assert.True(t, ok, "command must produce OCMClientReadyMsg, got %T", msg)
	assert.Error(t, ready.Err)
	assert.Nil(t, ready.Client)
}

// Saving the wizard must queue the OCM connect alongside PD init, flagged via
// ocmAuthPending so the UI reflects the in-flight connection.
func TestConfigSaved_TriggersOCMConnect(t *testing.T) {
	mock := createMockOCMClient()
	m := createConfigTestModel()
	m.ocmClient = nil
	m.ocmConnect = func() (ocm.OCMClient, error) { return mock, nil }

	result, cmd := m.Update(configSavedMsg{err: nil})
	updated := result.(model)

	assert.NotNil(t, cmd)
	assert.True(t, updated.ocmAuthPending, "OCM connect must be queued after config save")
}

func TestConfigSaved_NoOCMConnectWhenAlreadyConnected(t *testing.T) {
	m := createConfigTestModel()
	m.ocmClient = createMockOCMClient()

	result, cmd := m.Update(configSavedMsg{err: nil})
	updated := result.(model)

	assert.NotNil(t, cmd, "PD init must still be dispatched")
	assert.False(t, updated.ocmAuthPending, "no OCM connect when a client already exists")
}

func TestConfigSaved_NoOCMConnectOnSaveError(t *testing.T) {
	m := createConfigTestModel()
	m.ocmClient = nil
	m.ocmConnect = func() (ocm.OCMClient, error) { return createMockOCMClient(), nil }

	result, _ := m.Update(configSavedMsg{err: fmt.Errorf("disk full")})
	updated := result.(model)

	assert.False(t, updated.ocmAuthPending, "failed save must not trigger OCM connect")
}

// A brand-new user aborting the wizard has no usable session — do not pop
// browser auth at them on the way out.
func TestConfigAborted_NoOCMConnectWithoutPDConfig(t *testing.T) {
	m := createConfigTestModel()
	m.configMode = true
	m.ocmClient = nil
	m.config = nil
	m.ocmConnect = func() (ocm.OCMClient, error) { return createMockOCMClient(), nil }

	form := huh.NewForm(huh.NewGroup(huh.NewNote().Title("test")))
	form.Init()
	form.State = huh.StateAborted
	m.configForm = form

	result, cmd := switchConfigFocusMode(m, tea.KeyMsg{Type: tea.KeyEscape})
	updated := result.(model)

	assert.False(t, updated.configMode)
	assert.Nil(t, cmd)
	assert.False(t, updated.ocmAuthPending, "abort without a PD config must not connect OCM")
}
