package parser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/julienbreux/ayg-conv-to-fs/pkg/models"
)

// ParseResult holds the outcome of a transcript parsing operation.
type ParseResult struct {
	Steps         []models.Step
	LastStepIndex int
	BytesRead     int64
	Errors        []error
}

// TranscriptParser parses Antigravity JSONL transcripts.
type TranscriptParser struct{}

// NewTranscriptParser creates a new TranscriptParser instance.
func NewTranscriptParser() *TranscriptParser {
	return &TranscriptParser{}
}

// ParseFile parses an entire transcript JSONL file from the beginning.
func (p *TranscriptParser) ParseFile(path string) (*ParseResult, error) {
	return p.ParseFileFromOffset(path, 0)
}

// ParseFileFromOffset parses a transcript JSONL file starting from a specific byte offset.
func (p *TranscriptParser) ParseFileFromOffset(path string, startOffset int64) (*ParseResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open transcript file %s: %w", path, err)
	}
	defer func() {
		_ = f.Close()
	}()

	if startOffset > 0 {
		if _, err := f.Seek(startOffset, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to seek to offset %d in %s: %w", startOffset, path, err)
		}
	}

	result := &ParseResult{
		Steps:         make([]models.Step, 0),
		LastStepIndex: -1,
		BytesRead:     startOffset,
		Errors:        make([]error, 0),
	}

	reader := bufio.NewReader(f)

	for {
		line, err := reader.ReadBytes('\n')
		lineLen := int64(len(line))
		result.BytesRead += lineLen

		if len(line) > 0 {
			// Trim possible carriage returns
			trimmed := line
			if len(trimmed) > 0 && trimmed[len(trimmed)-1] == '\n' {
				trimmed = trimmed[:len(trimmed)-1]
			}
			if len(trimmed) > 0 && trimmed[len(trimmed)-1] == '\r' {
				trimmed = trimmed[:len(trimmed)-1]
			}

			if len(trimmed) > 0 {
				var step models.Step
				if errUnmarshal := json.Unmarshal(trimmed, &step); errUnmarshal != nil {
					result.Errors = append(result.Errors, fmt.Errorf("invalid json on line: %w", errUnmarshal))
				} else {
					result.Steps = append(result.Steps, step)
					if step.StepIndex > result.LastStepIndex {
						result.LastStepIndex = step.StepIndex
					}
				}
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("error reading transcript %s: %w", path, err)
		}
	}

	return result, nil
}

// SerializeStepToJSONL formats a Step object into a single-line JSON with trailing newline.
func SerializeStepToJSONL(step *models.Step) ([]byte, error) {
	if step == nil {
		return nil, fmt.Errorf("cannot serialize nil step")
	}
	data, err := json.Marshal(step)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal step: %w", err)
	}
	return append(data, '\n'), nil
}
