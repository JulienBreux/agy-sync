package discovery

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DiscoveredArtifact represents a file identified within a conversation folder.
type DiscoveredArtifact struct {
	RelativePath string
	AbsolutePath string
	SizeBytes    int64
	SHA256       string
	LastModified time.Time
}

// DiscoveredConversation represents an Antigravity conversation directory and its contents.
type DiscoveredConversation struct {
	ID             string
	Path           string
	TranscriptPath string
	HasTranscript  bool
	Artifacts      []DiscoveredArtifact
	LastModified   time.Time
}

// DiscoverConversations scans the specified brain directory for all conversation folders.
func DiscoverConversations(brainDir string) ([]DiscoveredConversation, error) {
	entries, err := os.ReadDir(brainDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read brain directory %s: %w", brainDir, err)
	}

	var results []DiscoveredConversation

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		convID := entry.Name()
		// Skip hidden directories at the root level if any
		if strings.HasPrefix(convID, ".") {
			continue
		}

		conv, err := DiscoverConversation(brainDir, convID)
		if err != nil {
			// Skip or continue if individual folder cannot be read
			continue
		}

		results = append(results, *conv)
	}

	return results, nil
}

// DiscoverConversation inspects a single conversation directory.
func DiscoverConversation(brainDir, convID string) (*DiscoveredConversation, error) {
	convPath := filepath.Join(brainDir, convID)
	info, err := os.Stat(convPath)
	if err != nil {
		return nil, fmt.Errorf("conversation directory not found at %s: %w", convPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path %s is not a directory", convPath)
	}

	// Standard Antigravity transcript location: .system_generated/logs/transcript.jsonl
	transcriptPath := filepath.Join(convPath, ".system_generated", "logs", "transcript.jsonl")
	hasTranscript := false
	if tInfo, err := os.Stat(transcriptPath); err == nil && !tInfo.IsDir() {
		hasTranscript = true
	}

	artifacts, err := ScanArtifacts(convPath)
	if err != nil {
		return nil, fmt.Errorf("failed scanning artifacts in %s: %w", convPath, err)
	}

	return &DiscoveredConversation{
		ID:             convID,
		Path:           convPath,
		TranscriptPath: transcriptPath,
		HasTranscript:  hasTranscript,
		Artifacts:      artifacts,
		LastModified:   info.ModTime(),
	}, nil
}

// ScanArtifacts traverses the conversation folder and collects all files except system internals.
func ScanArtifacts(convDir string) ([]DiscoveredArtifact, error) {
	var artifacts []DiscoveredArtifact

	err := filepath.WalkDir(convDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, errRel := filepath.Rel(convDir, path)
		if errRel != nil {
			return errRel
		}

		// Skip root directory
		if rel == "." {
			return nil
		}

		// Ignore .system_generated directory and its children
		if top, _, _ := strings.Cut(rel, string(filepath.Separator)); top == ".system_generated" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Only index files
		if d.IsDir() {
			return nil
		}

		info, errInfo := d.Info()
		if errInfo != nil {
			return errInfo
		}

		hashStr, errHash := computeSHA256(path)
		if errHash != nil {
			return fmt.Errorf("failed hashing %s: %w", path, errHash)
		}

		artifacts = append(artifacts, DiscoveredArtifact{
			RelativePath: rel,
			AbsolutePath: path,
			SizeBytes:    info.Size(),
			SHA256:       hashStr,
			LastModified: info.ModTime(),
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return artifacts, nil
}

func computeSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = f.Close()
	}()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
