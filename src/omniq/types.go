package omniq

type PayloadT any

type JobCtx struct {
	Queue        string
	JobID        string
	PayloadRaw  string
	Payload     PayloadT
	Attempt     int
	LockUntilMs int64
	LeaseToken  string
	GID         string
}

type ReserveStatus interface {
	isReserveStatus()
}

type ReservePaused struct {
	Status string
}

func (ReservePaused) isReserveStatus() {}

type ReserveJob struct {
	Status      string
	JobID       string
	Payload     string
	LockUntilMs int64
	Attempt     int
	GID         string
	LeaseToken  string
}

func (ReserveJob) isReserveStatus() {}

type ReserveResult = ReserveStatus

type AckFailStatus string

const (
	AckRetry  AckFailStatus = "RETRY"
	AckFailed AckFailStatus = "FAILED"
)

type AckFailResult struct {
	Status      AckFailStatus
	NextRunAtMs *int64
}
