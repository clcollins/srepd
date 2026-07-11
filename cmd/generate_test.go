package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunConfigGenerate_Stdout(t *testing.T) {
	var sb testWriter

	err := runConfigGenerate(&sb, "", false)

	assert.NoError(t, err)
	assert.Contains(t, sb.String(), `token: ""`)
}

func TestRunConfigGenerate_WritesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "srepd.yaml")

	err := runConfigGenerate(nil, path, false)
	assert.NoError(t, err)

	info, statErr := os.Stat(path)
	assert.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "generated config may hold a token later — 0600")

	data, readErr := os.ReadFile(path)
	assert.NoError(t, readErr)
	assert.Contains(t, string(data), "teams: []")
}

func TestRunConfigGenerate_RefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "srepd.yaml")
	assert.NoError(t, os.WriteFile(path, []byte("precious: data\n"), 0600))

	err := runConfigGenerate(nil, path, false)

	assert.Error(t, err, "must refuse to clobber an existing file without --force")
	data, _ := os.ReadFile(path)
	assert.Equal(t, "precious: data\n", string(data))
}

func TestRunConfigGenerate_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "srepd.yaml")
	assert.NoError(t, os.WriteFile(path, []byte("precious: data\n"), 0600))

	err := runConfigGenerate(nil, path, true)

	assert.NoError(t, err)
	data, _ := os.ReadFile(path)
	assert.Contains(t, string(data), `token: ""`)
}

type testWriter struct{ data []byte }

func (w *testWriter) Write(p []byte) (int, error) {
	w.data = append(w.data, p...)
	return len(p), nil
}

func (w *testWriter) String() string { return string(w.data) }
