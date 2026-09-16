package models_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/pkg/models"
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

func TestDBChunkSerialization(t *testing.T) {
	chunk := &models.DBChunk{
		ChunkIndex:  0,
		TotalChunks: 2,
		SizeBytes:   4,
		SHA256:      "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Data:        []byte("test"),
	}

	data, err := json.Marshal(chunk)
	require.NoError(t, err)

	var decoded models.DBChunk
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, 0, decoded.ChunkIndex)
	assert.Equal(t, 2, decoded.TotalChunks)
	assert.Equal(t, 4, decoded.SizeBytes)
	assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", decoded.SHA256)
	assert.Equal(t, []byte("test"), decoded.Data)
}

func TestConversationDBMetadata(t *testing.T) {
	now := time.Now().UTC()
	conv := &models.Conversation{
		ID:                     "conv-12345",
		Title:                  "Setup Conductor",
		Preview:                "Setup Conductor Preview",
		StepCount:              42,
		CreatedAt:              now,
		UpdatedAt:              now,
		LastSyncedStep:         5,
		SourceMachine:          "julien-macbook",
		DBSHA256:               "dummy-sha",
		DBSizeBytes:            1024,
		DBChunksCount:          1,
		LastUserInputTime:      now,
		LastUserInputStepIndex: 3,
		RawSummary:             []byte{0x01, 0x02, 0x03},
	}

	data, err := json.Marshal(conv)
	require.NoError(t, err)

	var decoded models.Conversation
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "Setup Conductor Preview", decoded.Preview)
	assert.Equal(t, 42, decoded.StepCount)
	assert.Equal(t, "dummy-sha", decoded.DBSHA256)
	assert.Equal(t, int64(1024), decoded.DBSizeBytes)
	assert.Equal(t, 1, decoded.DBChunksCount)
	assert.Equal(t, 3, decoded.LastUserInputStepIndex)
	assert.Equal(t, []byte{0x01, 0x02, 0x03}, decoded.RawSummary)
}
