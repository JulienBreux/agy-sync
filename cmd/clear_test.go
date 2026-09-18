package cmd_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/cmd"
	"github.com/julienbreux/agy-sync/internal/firestore"
	"github.com/julienbreux/agy-sync/internal/syncer"
	"github.com/julienbreux/agy-sync/pkg/config"
	"github.com/julienbreux/agy-sync/pkg/models"
)

func setupTestEnvironment(t *testing.T) (string, *firestore.MemoryRepository) {
	t.Helper()
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	cfgContent := `brain_dir: ` + tempDir + `
project_id: "test-clear-proj"
database_id: "(default)"
`
	require.NoError(t, os.WriteFile(configPath, []byte(cfgContent), 0o644))

	memRepo := firestore.NewMemoryRepository()
	t.Cleanup(func() {
		_ = memRepo.Close()
		cmd.ResetFirestoreClientFactory()
	})

	cmd.SetFirestoreClientFactory(func(_ context.Context, _ *config.Config) (firestore.Repository, error) {
		return memRepo, nil
	})

	return configPath, memRepo
}

func seedTestConversations(t *testing.T, repo firestore.Repository) {
	t.Helper()
	ctx := context.Background()
	c1 := &models.Conversation{ID: "conv-1", Title: "Conv 1", CreatedAt: time.Now()}
	c2 := &models.Conversation{ID: "conv-2", Title: "Conv 2", CreatedAt: time.Now()}
	require.NoError(t, repo.UpsertConversation(ctx, c1))
	require.NoError(t, repo.UpsertConversation(ctx, c2))
}

func TestClearCommand_InteractiveAbort(t *testing.T) {
	configPath, repo := setupTestEnvironment(t)
	seedTestConversations(t, repo)

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetIn(strings.NewReader("n\n"))

	root.SetArgs([]string{"clear", "--config", configPath})
	err := root.Execute()
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Are you sure you want to clear Firestore data")
	assert.Contains(t, out, "Operation cancelled")

	// Verify data is untouched
	convs, err := repo.ListConversations(context.Background())
	require.NoError(t, err)
	assert.Len(t, convs, 2)
}

func TestClearCommand_InteractiveUnexpectedInput(t *testing.T) {
	configPath, repo := setupTestEnvironment(t)
	seedTestConversations(t, repo)

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetIn(strings.NewReader("not-yes\n"))

	root.SetArgs([]string{"clear", "--config", configPath})
	err := root.Execute()
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Operation cancelled")

	// Verify data is untouched
	convs, err := repo.ListConversations(context.Background())
	require.NoError(t, err)
	assert.Len(t, convs, 2)
}

func TestClearCommand_InteractiveConfirm_Yes(t *testing.T) {
	configPath, repo := setupTestEnvironment(t)
	seedTestConversations(t, repo)

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetIn(strings.NewReader("yes\n"))

	root.SetArgs([]string{"clear", "--config", configPath})
	err := root.Execute()
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Are you sure you want to clear Firestore data")
	assert.Contains(t, out, "Successfully cleared 2 conversation(s)")

	// Verify data is cleared
	convs, err := repo.ListConversations(context.Background())
	require.NoError(t, err)
	assert.Empty(t, convs)
}

func TestClearCommand_Force_All(t *testing.T) {
	configPath, repo := setupTestEnvironment(t)
	seedTestConversations(t, repo)

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{"clear", "--force", "--config", configPath})
	err := root.Execute()
	require.NoError(t, err)

	out := buf.String()
	assert.NotContains(t, out, "Are you sure")
	assert.Contains(t, out, "Successfully cleared 2 conversation(s)")

	convs, err := repo.ListConversations(context.Background())
	require.NoError(t, err)
	assert.Empty(t, convs)
}

func TestClearCommand_Force_SingleConversation(t *testing.T) {
	configPath, repo := setupTestEnvironment(t)
	seedTestConversations(t, repo)

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{"clear", "--conversation", "conv-1", "-f", "--config", configPath})
	err := root.Execute()
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Successfully cleared conversation conv-1")

	c1, err := repo.GetConversation(context.Background(), "conv-1")
	require.NoError(t, err)
	assert.Nil(t, c1)

	c2, err := repo.GetConversation(context.Background(), "conv-2")
	require.NoError(t, err)
	assert.NotNil(t, c2)
}

func TestClearCommand_JSON(t *testing.T) {
	configPath, repo := setupTestEnvironment(t)
	seedTestConversations(t, repo)

	root := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(errBuf)

	root.SetArgs([]string{"clear", "-f", "--json", "--config", configPath})
	err := root.Execute()
	require.NoError(t, err)

	var res syncer.ClearResult
	require.NoError(t, json.Unmarshal(buf.Bytes(), &res))
	assert.Equal(t, 2, res.ConversationsDeleted)
	assert.Equal(t, "success", res.Status)
}
