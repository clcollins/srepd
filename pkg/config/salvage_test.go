package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// When viper can't parse the YAML (e.g. a missing key name before a sequence
// entry), we still want to salvage whatever scalar values we can so the
// wizard can pre-fill them — especially the token, which the user should
// never have to re-enter when it's right there in the file.

func TestSalvageConfigValues_ExtractsToken(t *testing.T) {
	broken := `---
# PagerDuty API token
token: u+tEnfytqWmndsYy-ghQ

# PagerDuty team IDs
  - PASPK4G

# Optional settings
editor: vim
terminal: gnome-terminal --
`
	vals := SalvageConfigValues([]byte(broken))

	assert.Equal(t, "u+tEnfytqWmndsYy-ghQ", vals["token"])
	assert.Equal(t, "vim", vals["editor"])
	assert.Equal(t, "gnome-terminal --", vals["terminal"])
}

func TestSalvageConfigValues_ValidYAML(t *testing.T) {
	valid := `token: mytoken
teams:
  - TEAM1
editor: nano
`
	vals := SalvageConfigValues([]byte(valid))

	assert.Equal(t, "mytoken", vals["token"])
	assert.Equal(t, "nano", vals["editor"])
}

func TestSalvageConfigValues_EmptyFile(t *testing.T) {
	vals := SalvageConfigValues([]byte(""))
	assert.Empty(t, vals)
}

func TestSalvageConfigValues_CommentsOnly(t *testing.T) {
	vals := SalvageConfigValues([]byte("# just comments\n# nothing here\n"))
	assert.Empty(t, vals)
}

func TestSalvageConfigValues_SkipsCommentedKeys(t *testing.T) {
	data := `token: real
# silent_policy: commented_out
editor: vim
`
	vals := SalvageConfigValues([]byte(data))
	assert.Equal(t, "real", vals["token"])
	assert.Equal(t, "vim", vals["editor"])
	_, hasSilent := vals["silent_policy"]
	assert.False(t, hasSilent)
}

func TestSalvageConfigValues_QuotedValues(t *testing.T) {
	data := `token: "u+quoted-token"
editor: 'nano'
`
	vals := SalvageConfigValues([]byte(data))
	assert.Equal(t, "u+quoted-token", vals["token"])
	assert.Equal(t, "nano", vals["editor"])
}

// The salvaged token must be usable for pre-filling the wizard's existing
// config, which means it needs to flow through ClassifyConfigHealth as a
// real token (not a placeholder).
func TestSalvageConfigValues_TokenIsNotPlaceholder(t *testing.T) {
	data := `token: u+realToken123
`
	vals := SalvageConfigValues([]byte(data))
	assert.False(t, HasPlaceholderToken(vals["token"]))
}
