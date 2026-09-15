package cmd_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/cmd"
)

func TestVersionCommand_Text(t *testing.T) {
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"version"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "agy-sync version")
	assert.Contains(t, out, "go: ")
	assert.Contains(t, out, "platform: ")
}

func TestVersionCommand_JSON(t *testing.T) {
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"version", "--json"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var payload map[string]any
	err = json.Unmarshal(buf.Bytes(), &payload)
	require.NoError(t, err)

	assert.Contains(t, payload, "version")
	assert.Contains(t, payload, "go_version")
	assert.Contains(t, payload, "os")
	assert.Contains(t, payload, "arch")
}
