package omniq

type PayloadT any

type JobCtx struct {
	Queue       string   `json:"queue"`
	JobID       string   `json:"job_id"`
	PayloadRaw  string   `json:"payload_raw"`
	Payload     PayloadT `json:"payload"`
	Attempt     int      `json:"attempt"`
	MaxAttempts int      `json:"max_attempts"`
	LockUntilMs int64    `json:"lock_until_ms"`
	LeaseToken  string   `json:"lease_token"`
	GID         string   `json:"gid"`
	Exec        *Exec    `json:"exec"`
}

type ReserveStatus interface {
	isReserveStatus()
}

type ReservePaused struct {
	Status string `json:"status"`
}

func (ReservePaused) isReserveStatus() {}

type ReserveJob struct {
	Status      string `json:"status"`
	JobID       string `json:"job_id"`
	Payload     string `json:"payload"`
	LockUntilMs int64  `json:"lock_until_ms"`
	Attempt     int    `json:"attempt"`
	MaxAttempts int    `json:"max_attempts"`
	GID         string `json:"gid"`
	LeaseToken  string `json:"lease_token"`
}

func (ReserveJob) isReserveStatus() {}

type ReserveResult = ReserveStatus

type AckFailStatus string

const (
	AckRetry  AckFailStatus = "RETRY"
	AckFailed AckFailStatus = "FAILED"
)

type AckFailResult struct {
	Status      AckFailStatus `json:"status"`
	NextRunAtMs *int64        `json:"next_run_at_ms"`
}

type BatchResult struct {
	JobID  string  `json:"job_id"`
	Status string  `json:"status"`
	Reason *string `json:"reason"`
}
