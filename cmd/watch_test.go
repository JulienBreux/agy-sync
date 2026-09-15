package cmd_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/cmd"
	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/pkg/firestore"
)

func TestWatchCommand_MissingConfig(t *testing.T) {
	tempDir := t.TempDir()
	nonExistentConfig := filepath.Join(tempDir, "missing.yaml")

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{"watch", "--config", nonExistentConfig})
	err := root.Execute()

	assert.Error(t, err)
}

func TestWatchCommand_GracefulShutdown(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	cfgContent := `brain_dir: ` + tempDir + `
project_id: "test-watch-proj"
machine_id: "test-watch-box"
`
	require.NoError(t, os.WriteFile(configPath, []byte(cfgContent), 0o644))

	memRepo := firestore.NewMemoryRepository()
	cmd.SetFirestoreClientFactory(func(_ context.Context, _ *config.Config) (firestore.Repository, error) {
		return memRepo, nil
	})
	defer cmd.ResetFirestoreClientFactory()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{"watch", "--config", configPath, "--interval", "50ms", "--debounce", "20ms"})

	err := root.ExecuteContext(ctx)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Starting real-time synchronization daemon")
}
