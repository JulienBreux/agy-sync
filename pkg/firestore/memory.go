package firestore

import (
	"context"
	"sort"
	"sync"

	"github.com/julienbreux/ayg-conv-to-fs/pkg/models"
)

// MemoryRepository is an in-memory thread-safe implementation of Repository for testing and local simulation.
type MemoryRepository struct {
	mu            sync.RWMutex
	conversations map[string]*models.Conversation
	steps         map[string]map[int]models.Step
	artifacts     map[string]map[string]*models.Artifact
}

// NewMemoryRepository constructs a new in-memory repository instance.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		conversations: make(map[string]*models.Conversation),
		steps:         make(map[string]map[int]models.Step),
		artifacts:     make(map[string]map[string]*models.Artifact),
	}
}

// Close releases any allocated resources.
func (m *MemoryRepository) Close() error {
	return nil
}

// UpsertConversation creates or updates a conversation record.
func (m *MemoryRepository) UpsertConversation(_ context.Context, conv *models.Conversation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cCopy := *conv
	m.conversations[conv.ID] = &cCopy
	return nil
}

// GetConversation retrieves a conversation by ID.
func (m *MemoryRepository) GetConversation(_ context.Context, id string) (*models.Conversation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conv, ok := m.conversations[id]
	if !ok {
		return nil, nil
	}
	cCopy := *conv
	return &cCopy, nil
}

// AppendSteps adds steps to the specified conversation monotonically.
func (m *MemoryRepository) AppendSteps(_ context.Context, convID string, steps []models.Step) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.steps[convID]; !ok {
		m.steps[convID] = make(map[int]models.Step)
	}

	for _, s := range steps {
		m.steps[convID][s.StepIndex] = s
	}

	return nil
}

// GetStepsSince returns steps with StepIndex > afterIndex ordered by StepIndex.
func (m *MemoryRepository) GetStepsSince(_ context.Context, convID string, afterIndex int) ([]models.Step, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stepMap, ok := m.steps[convID]
	if !ok {
		return []models.Step{}, nil
	}

	var results []models.Step
	for _, s := range stepMap {
		if s.StepIndex > afterIndex {
			results = append(results, s)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].StepIndex < results[j].StepIndex
	})

	return results, nil
}

// SaveArtifact persists an artifact record.
func (m *MemoryRepository) SaveArtifact(_ context.Context, artifact *models.Artifact) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.artifacts[artifact.ConversationID]; !ok {
		m.artifacts[artifact.ConversationID] = make(map[string]*models.Artifact)
	}

	aCopy := *artifact
	m.artifacts[artifact.ConversationID][artifact.ID] = &aCopy
	return nil
}

// GetArtifact retrieves a specific artifact.
func (m *MemoryRepository) GetArtifact(_ context.Context, convID, artifactID string) (*models.Artifact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	convArtifacts, ok := m.artifacts[convID]
	if !ok {
		return nil, nil
	}

	art, ok := convArtifacts[artifactID]
	if !ok {
		return nil, nil
	}

	aCopy := *art
	return &aCopy, nil
}

// ListArtifacts returns all artifacts for a given conversation.
func (m *MemoryRepository) ListArtifacts(_ context.Context, convID string) ([]models.Artifact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	convArtifacts, ok := m.artifacts[convID]
	if !ok {
		return []models.Artifact{}, nil
	}

	results := make([]models.Artifact, 0, len(convArtifacts))
	for _, art := range convArtifacts {
		results = append(results, *art)
	}

	return results, nil
}
