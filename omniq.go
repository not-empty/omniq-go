package omniq

import internal "github.com/not-empty/omniq-go/src/omniq"

// ===== Client =====
type Client = internal.Client
type ClientOpts = internal.ClientOpts
type ConsumeOpts = internal.ConsumeOpts
type JobCtx = internal.JobCtx
type PublishOpts = internal.PublishOpts

var NewClient = internal.NewClient

// ===== Reserve / ack / batch models =====
type ReserveResult = internal.ReserveResult
type ReservePaused = internal.ReservePaused
type ReserveJob = internal.ReserveJob
type AckFailResult = internal.AckFailResult
type BatchResult = internal.BatchResult

// ===== Monitor =====
type Monitor = internal.Monitor
type LaneName = internal.LaneName
type QueueStats = internal.QueueStats
type GroupReady = internal.GroupReady
type GroupStatus = internal.GroupStatus
type LaneJob = internal.LaneJob
type JobInfo = internal.JobInfo
type QueueOverview = internal.QueueOverview

var NewMonitor = internal.NewMonitor

const (
	LaneWait      = internal.LaneWait
	LaneActive    = internal.LaneActive
	LaneDelayed   = internal.LaneDelayed
	LaneFailed    = internal.LaneFailed
	LaneCompleted = internal.LaneCompleted
)
