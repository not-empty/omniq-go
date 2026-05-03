package omniq

type LaneName string

const (
	LaneWait      LaneName = "wait"
	LaneActive    LaneName = "active"
	LaneDelayed   LaneName = "delayed"
	LaneFailed    LaneName = "failed"
	LaneCompleted LaneName = "completed"
)

type QueueStats struct {
	Queue          string `json:"queue"`
	Paused         bool   `json:"paused"`
	Waiting        int64  `json:"waiting"`
	GroupWaiting   int64  `json:"group_waiting"`
	WaitingTotal   int64  `json:"waiting_total"`
	Active         int64  `json:"active"`
	Delayed        int64  `json:"delayed"`
	Failed         int64  `json:"failed"`
	CompletedKept  int64  `json:"completed_kept"`
	GroupsReady    int64  `json:"groups_ready"`
	LastActivityMS int64  `json:"last_activity_ms"`
	LastEnqueueMS  int64  `json:"last_enqueue_ms"`
	LastReserveMS  int64  `json:"last_reserve_ms"`
	LastFinishMS   int64  `json:"last_finish_ms"`
}

type GroupReady struct {
	GID     string `json:"gid"`
	ScoreMS int64  `json:"score_ms"`
}

type GroupStatus struct {
	GID          string `json:"gid"`
	Inflight     int64  `json:"inflight"`
	Limit        int64  `json:"limit"`
	Ready        bool   `json:"ready"`
	WaitingCount int64  `json:"waiting_count"`
}

type LaneJob struct {
	Lane           LaneName `json:"lane"`
	JobID          string   `json:"job_id"`
	IdxScoreMS     int64    `json:"idx_score_ms"`
	State          string   `json:"state"`
	GID            string   `json:"gid"`
	Attempt        int64    `json:"attempt"`
	MaxAttempts    int64    `json:"max_attempts"`
	DueMS          int64    `json:"due_ms"`
	LockUntilMS    int64    `json:"lock_until_ms"`
	QueuedMS       int64    `json:"queued_ms"`
	FirstStartedMS int64    `json:"first_started_ms"`
	LastStartedMS  int64    `json:"last_started_ms"`
	CompletedMS    int64    `json:"completed_ms"`
	FailedMS       int64    `json:"failed_ms"`
	UpdatedMS      int64    `json:"updated_ms"`
	LastError      string   `json:"last_error"`
}

type JobInfo struct {
	JobID          string `json:"job_id"`
	State          string `json:"state"`
	GID            string `json:"gid"`
	Attempt        int64  `json:"attempt"`
	MaxAttempts    int64  `json:"max_attempts"`
	TimeoutMS      int64  `json:"timeout_ms"`
	BackoffMS      int64  `json:"backoff_ms"`
	LeaseToken     string `json:"lease_token"`
	LockUntilMS    int64  `json:"lock_until_ms"`
	DueMS          int64  `json:"due_ms"`
	Payload        string `json:"payload"`
	LastError      string `json:"last_error"`
	LastErrorMS    int64  `json:"last_error_ms"`
	CreatedMS      int64  `json:"created_ms"`
	UpdatedMS      int64  `json:"updated_ms"`
	QueuedMS       int64  `json:"queued_ms"`
	FirstStartedMS int64  `json:"first_started_ms"`
	LastStartedMS  int64  `json:"last_started_ms"`
	CompletedMS    int64  `json:"completed_ms"`
	FailedMS       int64  `json:"failed_ms"`
}

type QueueOverview struct {
	Stats       QueueStats   `json:"stats"`
	ReadyGroups []GroupReady `json:"ready_groups"`
	Active      []LaneJob    `json:"active"`
	Delayed     []LaneJob    `json:"delayed"`
	Failed      []LaneJob    `json:"failed"`
	Completed   []LaneJob    `json:"completed"`
}
