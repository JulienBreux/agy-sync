package models

// TrajectoryMeta represents a row in the trajectory_meta table in an Antigravity conversation SQLite database.
type TrajectoryMeta struct {
	TrajectoryID   string `json:"trajectory_id" firestore:"trajectory_id"`
	CascadeID      string `json:"cascade_id" firestore:"cascade_id"`
	TrajectoryType int    `json:"trajectory_type" firestore:"trajectory_type"`
	Source         int    `json:"source" firestore:"source"`
}

// ConversationDBStep represents a row in the steps table in an Antigravity conversation SQLite database.
type ConversationDBStep struct {
	Idx              int    `json:"idx" firestore:"idx"`
	StepType         int    `json:"step_type" firestore:"step_type"`
	Status           int    `json:"status" firestore:"status"`
	HasSubtrajectory bool   `json:"has_subtrajectory" firestore:"has_subtrajectory"`
	Metadata         []byte `json:"metadata,omitempty" firestore:"metadata,omitempty"`
	ErrorDetails     []byte `json:"error_details,omitempty" firestore:"error_details,omitempty"`
	Permissions      []byte `json:"permissions,omitempty" firestore:"permissions,omitempty"`
	TaskDetails      []byte `json:"task_details,omitempty" firestore:"task_details,omitempty"`
	RenderInfo       []byte `json:"render_info,omitempty" firestore:"render_info,omitempty"`
	StepPayload      []byte `json:"step_payload,omitempty" firestore:"step_payload,omitempty"`
	StepFormat       int    `json:"step_format" firestore:"step_format"`
}

// GenMetadata represents a row in the gen_metadata table in an Antigravity conversation SQLite database.
type GenMetadata struct {
	Idx  int    `json:"idx" firestore:"idx"`
	Data []byte `json:"data,omitempty" firestore:"data,omitempty"`
	Size int    `json:"size" firestore:"size"`
}

// ExecutorMetadata represents a row in the executor_metadata table in an Antigravity conversation SQLite database.
type ExecutorMetadata struct {
	Idx  int    `json:"idx" firestore:"idx"`
	Data []byte `json:"data,omitempty" firestore:"data,omitempty"`
}

// ParentReference represents a row in the parent_references table in an Antigravity conversation SQLite database.
type ParentReference struct {
	Idx  int    `json:"idx" firestore:"idx"`
	Data []byte `json:"data,omitempty" firestore:"data,omitempty"`
}

// TrajectoryMetadataBlob represents a row in the trajectory_metadata_blob table in an Antigravity conversation SQLite database.
type TrajectoryMetadataBlob struct {
	ID   string `json:"id" firestore:"id"`
	Data []byte `json:"data,omitempty" firestore:"data,omitempty"`
}

// BattleModeInfo represents a row in the battle_mode_infos table in an Antigravity conversation SQLite database.
type BattleModeInfo struct {
	Idx  int    `json:"idx" firestore:"idx"`
	Data []byte `json:"data,omitempty" firestore:"data,omitempty"`
}
