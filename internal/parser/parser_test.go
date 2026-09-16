package parser_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/internal/parser"
	"github.com/julienbreux/agy-sync/pkg/models"
)

const sampleTranscriptJSONL = `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-09-15T09:40:46Z","content":"Hello world"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-09-15T09:40:47Z","thinking":"Evaluating options...","tool_calls":[{"name":"view_file","args":{"AbsolutePath":"/path/to/doc"}}]}
{"step_index":2,"source":"SYSTEM","type":"SYSTEM_RESPONSE","status":"DONE","created_at":"2026-09-15T09:40:48Z","content":"Operation succeeded"}
`

func TestParseTranscriptFile(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "transcript.jsonl")
	err := os.WriteFile(logPath, []byte(sampleTranscriptJSONL), 0o644)
	require.NoError(t, err)

	p := parser.NewTranscriptParser()
	result, err := p.ParseFile(logPath)
	require.NoError(t, err)

	require.Len(t, result.Steps, 3)
	assert.Equal(t, 0, result.Steps[0].StepIndex)
	assert.Equal(t, "Hello world", result.Steps[0].Content)
	assert.Equal(t, 1, result.Steps[1].StepIndex)
	assert.Equal(t, "Evaluating options...", result.Steps[1].Thinking)
	assert.Equal(t, 2, result.Steps[2].StepIndex)
	assert.Equal(t, "Operation succeeded", result.Steps[2].Content)
	assert.Equal(t, 2, result.LastStepIndex)
	assert.Positive(t, result.BytesRead)
}

func TestIncrementalParseFromOffset(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "transcript.jsonl")
	err := os.WriteFile(logPath, []byte(sampleTranscriptJSONL), 0o644)
	require.NoError(t, err)

	p := parser.NewTranscriptParser()

	// Initial parse
	initial, err := p.ParseFile(logPath)
	require.NoError(t, err)
	require.Len(t, initial.Steps, 3)

	// Append a new line (step 3)
	newLine := `{"step_index":3,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-09-15T09:41:00Z","content":"New request"}` + "\n"
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	_, err = f.WriteString(newLine)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	// Parse only incremental changes from previous byte offset
	incremental, err := p.ParseFileFromOffset(logPath, initial.BytesRead)
	require.NoError(t, err)
	require.Len(t, incremental.Steps, 1)
	assert.Equal(t, 3, incremental.Steps[0].StepIndex)
	assert.Equal(t, "New request", incremental.Steps[0].Content)
	assert.Equal(t, 3, incremental.LastStepIndex)
}

func TestParseInvalidJSONLine(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "transcript.jsonl")
	corruptContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-09-15T09:40:46Z","content":"Valid line"}
{this is not valid json}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-09-15T09:40:47Z","content":"Another valid line"}
`
	err := os.WriteFile(logPath, []byte(corruptContent), 0o644)
	require.NoError(t, err)

	p := parser.NewTranscriptParser()
	result, err := p.ParseFile(logPath)
	require.NoError(t, err)
	// Should skip or record errors but recover valid lines
	require.Len(t, result.Steps, 2)
	assert.Equal(t, 0, result.Steps[0].StepIndex)
	assert.Equal(t, 1, result.Steps[1].StepIndex)
	assert.Len(t, result.Errors, 1)
}

func TestSerializeStep(t *testing.T) {
	step := &models.Step{
		StepIndex: 0,
		Source:    "USER_EXPLICIT",
		Type:      "USER_INPUT",
		Status:    "DONE",
		CreatedAt: time.Date(2026, 9, 15, 9, 40, 46, 0, time.UTC),
		Content:   "Testing serialization",
	}

	line, err := parser.SerializeStepToJSONL(step)
	require.NoError(t, err)
	assert.Contains(t, string(line), `"step_index":0`)
	assert.Contains(t, string(line), `"Testing serialization"`)
	assert.Equal(t, byte('\n'), line[len(line)-1])
}
