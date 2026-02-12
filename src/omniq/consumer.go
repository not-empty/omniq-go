package omniq

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

type heartbeatHandle struct {
	stopCh chan struct{}
	lost   atomic.Bool
	doneCh chan struct{}
}

func startHeartbeater(
	ops *OmniqOps,
	queue string,
	jobID string,
	leaseToken string,
	intervalS float64,
) *heartbeatHandle {
	h := &heartbeatHandle{
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}

	markLostIfLease := func(err error) bool {
		if err == nil {
			return false
		}
		msg := err.Error()
		if strings.Contains(msg, "NOT_ACTIVE") || strings.Contains(msg, "TOKEN_MISMATCH") {
			h.lost.Store(true)
			return true
		}
		return false
	}

	go func() {
		defer close(h.doneCh)

		if _, err := ops.Heartbeat(queue, jobID, leaseToken, 0); err != nil {
			if markLostIfLease(err) {
				return
			}
		}

		if intervalS <= 0 {
			intervalS = 1.0
		}

		t := time.NewTicker(time.Duration(intervalS * float64(time.Second)))
		defer t.Stop()

		for {
			select {
			case <-h.stopCh:
				return
			case <-t.C:
				if _, err := ops.Heartbeat(queue, jobID, leaseToken, 0); err != nil {
					if markLostIfLease(err) {
						return
					}
				}
			}
		}
	}()

	return h
}

func safeLog(logger ConsumeLogger, msg string) {
	defer func() { _ = recover() }()
	logger(msg)
}

func payloadPreview(payload any, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 300
	}

	var s string
	switch t := payload.(type) {
	case string:
		s = t
	default:
		b, err := json.Marshal(t)
		if err != nil {
			s = fmt.Sprint(payload)
		} else {
			s = string(b)
		}
	}

	if len(s) > maxLen {
		return s[:maxLen] + "…"
	}
	return s
}

func consumeLoop(ops *OmniqOps, opts ConsumeOpts) error {
	applyConsumeDefaults(&opts)

	stopRequested, sigintCount := startSignalLoop(opts)

	var lastPromote time.Time
	var lastReap time.Time

	for {
		if stopRequested.Load() {
			if opts.Verbose {
				safeLog(opts.Logger, fmt.Sprintf("[consume] stop requested; exiting (idle). queue=%s", opts.Queue))
			}
			return nil
		}

		now := time.Now()
		maybePromote(ops, opts, now, &lastPromote)
		maybeReap(ops, opts, now, &lastReap)

		res, err := ops.Reserve(opts.Queue, 0)
		if err != nil {
			if opts.Verbose {
				safeLog(opts.Logger, fmt.Sprintf("[consume] reserve error: %v", err))
			}
			time.Sleep(200 * time.Millisecond)
			continue
		}

		if res == nil {
			time.Sleep(time.Duration(opts.PollIntervalS * float64(time.Second)))
			continue
		}

		switch v := res.(type) {
		case ReservePaused:
			time.Sleep(time.Duration(PausedBackoffS(opts.PollIntervalS) * float64(time.Second)))
			continue

		case ReserveJob:
			exitNow := handleReservedJob(ops, opts, v, stopRequested, sigintCount)
			if exitNow {
				return nil
			}

		default:
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func applyConsumeDefaults(opts *ConsumeOpts) {
	if opts.PollIntervalS <= 0 {
		opts.PollIntervalS = 0.05
	}
	if opts.PromoteIntervalS <= 0 {
		opts.PromoteIntervalS = 1.0
	}
	if opts.PromoteBatch <= 0 {
		opts.PromoteBatch = 1000
	}
	if opts.ReapIntervalS <= 0 {
		opts.ReapIntervalS = 1.0
	}
	if opts.ReapBatch <= 0 {
		opts.ReapBatch = 1000
	}
	if opts.Logger == nil {
		opts.Logger = DefaultConsumeLogger
	}
}

func startSignalLoop(opts ConsumeOpts) (*atomic.Bool, *atomic.Int32) {
	var stopRequested atomic.Bool
	var sigintCount atomic.Int32

	sigCh := make(chan os.Signal, 4)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		for sig := range sigCh {
			switch sig {
			case syscall.SIGTERM:
				stopRequested.Store(true)
				if opts.Verbose {
					safeLog(opts.Logger, fmt.Sprintf("[consume] SIGTERM received; stopping... queue=%s", opts.Queue))
				}

			case os.Interrupt:
				n := sigintCount.Add(1)
				if !opts.Drain {
					if opts.Verbose {
						safeLog(opts.Logger, fmt.Sprintf("[consume] SIGINT; hard exit now (drain=false). queue=%s", opts.Queue))
					}
					os.Exit(130)
				}
				if n >= 2 {
					if opts.Verbose {
						safeLog(opts.Logger, fmt.Sprintf("[consume] SIGINT x2; hard exit now. queue=%s", opts.Queue))
					}
					os.Exit(130)
				}

				stopRequested.Store(true)
				if opts.Verbose {
					safeLog(opts.Logger, fmt.Sprintf("[consume] Ctrl+C received; draining current job then exiting. queue=%s", opts.Queue))
				}
			}
		}
	}()

	return &stopRequested, &sigintCount
}

func maybePromote(ops *OmniqOps, opts ConsumeOpts, now time.Time, last *time.Time) {
	interval := time.Duration(opts.PromoteIntervalS * float64(time.Second))
	if last.IsZero() || now.Sub(*last) >= interval {
		_, _ = ops.PromoteDelayed(opts.Queue, opts.PromoteBatch, 0)
		*last = now
	}
}

func maybeReap(ops *OmniqOps, opts ConsumeOpts, now time.Time, last *time.Time) {
	interval := time.Duration(opts.ReapIntervalS * float64(time.Second))
	if last.IsZero() || now.Sub(*last) >= interval {
		_, _ = ops.ReapExpired(opts.Queue, opts.ReapBatch, 0)
		*last = now
	}
}

func handleReservedJob(
	ops *OmniqOps,
	opts ConsumeOpts,
	v ReserveJob,
	stopRequested *atomic.Bool,
	_ *atomic.Int32,
) bool {
	if strings.TrimSpace(v.LeaseToken) == "" {
		if opts.Verbose {
			safeLog(opts.Logger, fmt.Sprintf("[consume] invalid reserve (missing lease_token) job_id=%s", v.JobID))
		}
		time.Sleep(200 * time.Millisecond)
		return false
	}
	if stopRequested.Load() && !opts.Drain {
		if opts.Verbose {
			safeLog(opts.Logger, fmt.Sprintf("[consume] stop requested; fast-exit after reserve job_id=%s", v.JobID))
		}
		return true
	}

	ctx := buildJobCtx(ops, opts.Queue, v)

	if opts.Verbose {
		logReceived(opts, ctx)
	}

	hbS := deriveHBInterval(ops, opts, v.JobID)
	hb := startHeartbeater(ops, opts.Queue, v.JobID, v.LeaseToken, hbS)

	var recovered any

	func() {
		defer func() {
			if r := recover(); r != nil {
				recovered = r
			}
		}()
		opts.Handler(ctx)
	}()


	stopHeartbeater(hb)

	if recovered == nil {
		ackSuccess(ops, opts, v, ctx, hb)
	} else {
		ackFail(ops, opts, v, ctx, hb, recovered)
	}

	if stopRequested.Load() && opts.Drain {
		if opts.Verbose {
			safeLog(opts.Logger, fmt.Sprintf("[consume] stop requested; exiting after draining job_id=%s", ctx.JobID))
		}
		return true
	}

	return false
}

func buildJobCtx(ops *OmniqOps, queue string, v ReserveJob) JobCtx {
	var payloadObj any
	if err := json.Unmarshal([]byte(v.Payload), &payloadObj); err != nil {
		payloadObj = v.Payload
	}

	return JobCtx{
		Queue:       queue,
		JobID:       v.JobID,
		PayloadRaw:  v.Payload,
		Payload:     payloadObj,
		Attempt:     v.Attempt,
		LockUntilMs: v.LockUntilMs,
		LeaseToken:  v.LeaseToken,
		GID:         v.GID,
		CheckCompletion: newCheckCompletion(ops, v.JobID),
	}
}

func logReceived(opts ConsumeOpts, ctx JobCtx) {
	pv := payloadPreview(ctx.Payload, 300)
	gidS := ctx.GID
	if strings.TrimSpace(gidS) == "" {
		gidS = "-"
	}
	safeLog(opts.Logger, fmt.Sprintf(
		"[consume] received job_id=%s attempt=%d gid=%s payload=%s",
		ctx.JobID, ctx.Attempt, gidS, pv,
	))
}

func deriveHBInterval(ops *OmniqOps, opts ConsumeOpts, jobID string) float64 {
	if opts.HeartbeatIntervalS != nil {
		return *opts.HeartbeatIntervalS
	}
	tmo, _ := ops.JobTimeout(opts.Queue, jobID, 60_000)
	return DeriveHeartbeatIntervalS(tmo)
}

func stopHeartbeater(hb *heartbeatHandle) {
	close(hb.stopCh)
	select {
	case <-hb.doneCh:
	case <-time.After(100 * time.Millisecond):
	}
}

func ackSuccess(ops *OmniqOps, opts ConsumeOpts, v ReserveJob, ctx JobCtx, hb *heartbeatHandle) {
	if hb.lost.Load() {
		return
	}
	if err := ops.AckSuccess(opts.Queue, v.JobID, v.LeaseToken, 0); err != nil {
		if opts.Verbose {
			safeLog(opts.Logger, fmt.Sprintf("[consume] ack success error job_id=%s: %v", ctx.JobID, err))
		}
		return
	}
	if opts.Verbose {
		safeLog(opts.Logger, fmt.Sprintf("[consume] ack success job_id=%s", ctx.JobID))
	}
}

func ackFail(ops *OmniqOps, opts ConsumeOpts, v ReserveJob, ctx JobCtx, hb *heartbeatHandle, recovered any) {
    if hb.lost.Load() {
        return
    }

    var errS string
    switch t := recovered.(type) {
    case error:
        errS = fmt.Sprintf("%T: %v", t, t)
    case string:
        errS = "panic: " + t
    default:
        errS = fmt.Sprintf("panic: %T: %v", recovered, recovered)
    }

    res2, err2 := ops.AckFail(opts.Queue, v.JobID, v.LeaseToken, &errS, 0)

    if !opts.Verbose {
        return
    }
    if err2 != nil {
        safeLog(opts.Logger, fmt.Sprintf("[consume] ack fail error job_id=%s: %v", ctx.JobID, err2))
        return
    }

    if res2.Status == AckRetry && res2.NextRunAtMs != nil {
        safeLog(opts.Logger, fmt.Sprintf("[consume] ack fail job_id=%s => RETRY due_ms=%d", ctx.JobID, *res2.NextRunAtMs))
    } else {
        safeLog(opts.Logger, fmt.Sprintf("[consume] ack fail job_id=%s => FAILED", ctx.JobID))
    }
    safeLog(opts.Logger, fmt.Sprintf("[consume] error job_id=%s => %s", ctx.JobID, errS))
}

