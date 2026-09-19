package firestore

import (
	"cmp"
	"context"
	"errors"
	"maps"
	"slices"
	"sync"

	"github.com/julienbreux/agy-sync/pkg/models"
)

// MemoryRepository is an in-memory thread-safe implementation of Repository for testing and local simulation.
type MemoryRepository struct {
	mu            sync.RWMutex
	conversations map[string]*models.Conversation
	steps         map[string]map[int]models.Step
	artifacts     map[string]map[string]*models.Artifact
	dbChunks      map[string]map[int]models.DBChunk
}

// NewMemoryRepository constructs a new in-memory repository instance.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		conversations: make(map[string]*models.Conversation),
		steps:         make(map[string]map[int]models.Step),
		artifacts:     make(map[string]map[string]*models.Artifact),
		dbChunks:      make(map[string]map[int]models.DBChunk),
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

// ListConversations returns all stored conversations.
func (m *MemoryRepository) ListConversations(_ context.Context) ([]*models.Conversation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make([]*models.Conversation, 0, len(m.conversations))
	for _, conv := range m.conversations {
		copyConv := *conv
		res = append(res, &copyConv)
	}
	return res, nil
}

// AppendSteps adds steps to the specified conversation monotonically.
func (m *MemoryRepository) AppendSteps(_ context.Context, convID string, steps []models.Step) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.steps[convID]; !ok {
		m.steps[convID] = make(map[int]models.Step)
	}

	lastIndex := -1
	for _, s := range steps {
		m.steps[convID][s.StepIndex] = s
		lastIndex = max(lastIndex, s.StepIndex)
	}

	if conv, ok := m.conversations[convID]; ok {
		conv.LastSyncedStep = max(conv.LastSyncedStep, lastIndex)
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

	slices.SortFunc(results, func(a, b models.Step) int {
		return cmp.Compare(a.StepIndex, b.StepIndex)
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
	for art := range maps.Values(convArtifacts) {
		results = append(results, *art)
	}

	return results, nil
}

// SaveDBChunks saves database chunks for a conversation in memory.
func (m *MemoryRepository) SaveDBChunks(_ context.Context, convID string, chunks []models.DBChunk) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.dbChunks[convID]; !ok {
		m.dbChunks[convID] = make(map[int]models.DBChunk)
	}

	for _, chunk := range chunks {
		m.dbChunks[convID][chunk.ChunkIndex] = chunk
	}

	return nil
}

// GetDBChunks retrieves all database chunks for a conversation sorted by ChunkIndex.
func (m *MemoryRepository) GetDBChunks(_ context.Context, convID string) ([]models.DBChunk, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chunkMap, ok := m.dbChunks[convID]
	if !ok {
		return []models.DBChunk{}, nil
	}

	results := slices.Collect(maps.Values(chunkMap))

	slices.SortFunc(results, func(a, b models.DBChunk) int {
		return cmp.Compare(a.ChunkIndex, b.ChunkIndex)
	})

	return results, nil
}

// DeleteConversation removes a conversation and all its associated steps, artifacts, and dbChunks.
func (m *MemoryRepository) DeleteConversation(_ context.Context, convID string) error {
	if convID == "" {
		return errors.New("conversation_id cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.conversations, convID)
	delete(m.steps, convID)
	delete(m.artifacts, convID)
	delete(m.dbChunks, convID)

	return nil
}

// ClearAll removes all conversations and all associated subcollections.
func (m *MemoryRepository) ClearAll(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	clear(m.conversations)
	clear(m.steps)
	clear(m.artifacts)
	clear(m.dbChunks)

	return nil
}
