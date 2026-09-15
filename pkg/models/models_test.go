package models_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/ayg-conv-to-fs/pkg/models"
)

func TestStepSerialization(t *testing.T) {
	rawJSON := `{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-09-15T09:40:46Z","thinking":"Thinking about conductor rules...","tool_calls":[{"name":"view_file","args":{"AbsolutePath":"/path/to/file"}}],"truncated_fields":["content"]}`

	var step models.Step
	err := json.Unmarshal([]byte(rawJSON), &step)
	require.NoError(t, err)

	assert.Equal(t, 1, step.StepIndex)
	assert.Equal(t, "MODEL", step.Source)
	assert.Equal(t, "PLANNER_RESPONSE", step.Type)
	assert.Equal(t, "DONE", step.Status)
	assert.Equal(t, "Thinking about conductor rules...", step.Thinking)
	assert.Equal(t, "2026-09-15T09:40:46Z", step.CreatedAt.Format(time.RFC3339))
	require.Len(t, step.ToolCalls, 1)
	assert.Equal(t, "view_file", step.ToolCalls[0].Name)
	assert.Equal(t, "/path/to/file", step.ToolCalls[0].Args["AbsolutePath"])
	assert.Equal(t, []string{"content"}, step.TruncatedFields)
}

func TestConversationValidation(t *testing.T) {
	conv := &models.Conversation{
		ID:             "conv-12345",
		Title:          "Setup Conductor",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		LastSyncedStep: 5,
		SourceMachine:  "julien-macbook",
	}

	assert.NotEmpty(t, conv.ID)
	assert.Equal(t, "conv-12345", conv.ID)
	assert.Equal(t, 5, conv.LastSyncedStep)
}
