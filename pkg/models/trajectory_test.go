package models_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/pkg/models"
)

func TestTrajectoryMetaSerialization(t *testing.T) {
	meta := &models.TrajectoryMeta{
		TrajectoryID:   "traj-123",
		CascadeID:      "casc-456",
		TrajectoryType: 4,
		Source:         17,
	}

	data, err := json.Marshal(meta)
	require.NoError(t, err)

	var decoded models.TrajectoryMeta
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "traj-123", decoded.TrajectoryID)
	assert.Equal(t, "casc-456", decoded.CascadeID)
	assert.Equal(t, 4, decoded.TrajectoryType)
	assert.Equal(t, 17, decoded.Source)
}

func TestConversationDBStepSerialization(t *testing.T) {
	step := &models.ConversationDBStep{
		Idx:              1,
		StepType:         14,
		Status:           3,
		HasSubtrajectory: true,
		Metadata:         []byte("metadata-bytes"),
		ErrorDetails:     []byte("error-details-bytes"),
		Permissions:      []byte("permissions-bytes"),
		TaskDetails:      []byte("task-details-bytes"),
		RenderInfo:       []byte("render-info-bytes"),
		StepPayload:      []byte("step-payload-bytes"),
		StepFormat:       1,
	}

	data, err := json.Marshal(step)
	require.NoError(t, err)

	var decoded models.ConversationDBStep
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, 1, decoded.Idx)
	assert.Equal(t, 14, decoded.StepType)
	assert.Equal(t, 3, decoded.Status)
	assert.True(t, decoded.HasSubtrajectory)
	assert.Equal(t, []byte("metadata-bytes"), decoded.Metadata)
	assert.Equal(t, []byte("error-details-bytes"), decoded.ErrorDetails)
	assert.Equal(t, []byte("permissions-bytes"), decoded.Permissions)
	assert.Equal(t, []byte("task-details-bytes"), decoded.TaskDetails)
	assert.Equal(t, []byte("render-info-bytes"), decoded.RenderInfo)
	assert.Equal(t, []byte("step-payload-bytes"), decoded.StepPayload)
	assert.Equal(t, 1, decoded.StepFormat)
}

func TestAuxiliaryTrajectoryModelsSerialization(t *testing.T) {
	// GenMetadata
	gen := &models.GenMetadata{
		Idx:  5,
		Data: []byte("gen-data"),
		Size: 8,
	}
	data, err := json.Marshal(gen)
	require.NoError(t, err)
	var decodedGen models.GenMetadata
	err = json.Unmarshal(data, &decodedGen)
	require.NoError(t, err)
	assert.Equal(t, 5, decodedGen.Idx)
	assert.Equal(t, []byte("gen-data"), decodedGen.Data)
	assert.Equal(t, 8, decodedGen.Size)

	// ExecutorMetadata
	exec := &models.ExecutorMetadata{
		Idx:  10,
		Data: []byte("exec-data"),
	}
	data, err = json.Marshal(exec)
	require.NoError(t, err)
	var decodedExec models.ExecutorMetadata
	err = json.Unmarshal(data, &decodedExec)
	require.NoError(t, err)
	assert.Equal(t, 10, decodedExec.Idx)
	assert.Equal(t, []byte("exec-data"), decodedExec.Data)

	// ParentReference
	parentRef := &models.ParentReference{
		Idx:  2,
		Data: []byte("parent-ref-data"),
	}
	data, err = json.Marshal(parentRef)
	require.NoError(t, err)
	var decodedParent models.ParentReference
	err = json.Unmarshal(data, &decodedParent)
	require.NoError(t, err)
	assert.Equal(t, 2, decodedParent.Idx)
	assert.Equal(t, []byte("parent-ref-data"), decodedParent.Data)

	// TrajectoryMetadataBlob
	trajBlob := &models.TrajectoryMetadataBlob{
		ID:   "main",
		Data: []byte("blob-data"),
	}
	data, err = json.Marshal(trajBlob)
	require.NoError(t, err)
	var decodedBlob models.TrajectoryMetadataBlob
	err = json.Unmarshal(data, &decodedBlob)
	require.NoError(t, err)
	assert.Equal(t, "main", decodedBlob.ID)
	assert.Equal(t, []byte("blob-data"), decodedBlob.Data)

	// BattleModeInfo
	battle := &models.BattleModeInfo{
		Idx:  7,
		Data: []byte("battle-mode-data"),
	}
	data, err = json.Marshal(battle)
	require.NoError(t, err)
	var decodedBattle models.BattleModeInfo
	err = json.Unmarshal(data, &decodedBattle)
	require.NoError(t, err)
	assert.Equal(t, 7, decodedBattle.Idx)
	assert.Equal(t, []byte("battle-mode-data"), decodedBattle.Data)
}
