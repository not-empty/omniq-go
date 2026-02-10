package omniq

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
)

var scriptLock sync.Mutex

type OmniqOps struct {
	R       RedisLike
	Scripts OmniqScripts
}

func (o *OmniqOps) evalShaWithNoScriptFallback(sha, src string, numkeys int, keysAndArgs ...any) (any, error) {
	res, err := o.R.EvalSha(sha, numkeys, keysAndArgs...)
	if err == nil {
		return res, nil
	}

	if strings.Contains(strings.ToUpper(err.Error()), "NOSCRIPT") {
		scriptLock.Lock()
		defer scriptLock.Unlock()
		return o.R.Eval(src, numkeys, keysAndArgs...)
	}

	return nil, err
}

type PublishOpts struct {
	Queue         string
	Payload       any
	JobID         string
	MaxAttempts   int
	TimeoutMs     int64
	BackoffMs     int64
	DueMs         int64
	NowMsOverride int64
	GID           string
	GroupLimit    int
}

func (o *OmniqOps) Publish(opts PublishOpts) (string, error) {
	// Validate required inputs (match Python "named args" safety).
	if strings.TrimSpace(opts.Queue) == "" {
		return "", errors.New("publish(queue=...) is required")
	}
	if opts.Payload == nil {
		return "", errors.New("publish(payload=...) is required")
	}

	// Strict parity with Python: payload must be structured JSON (dict/list).
	if !isJSONStructured(opts.Payload) {
		return "", errors.New(
			"publish(payload=...) must be a dict or list (structured JSON). " +
				"Wrap strings as {'text': '...'} or {'value': '...'}.",
		)
	}

	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 3
	}
	if opts.TimeoutMs <= 0 {
		opts.TimeoutMs = 30_000
	}
	if opts.BackoffMs <= 0 {
		opts.BackoffMs = 5_000
	}

	anchor := QueueAnchor(opts.Queue)

	nms := opts.NowMsOverride
	if nms <= 0 {
		nms = NowMs()
	}

	jid := strings.TrimSpace(opts.JobID)
	if jid == "" {
		jid = NewULID()
	}

	payloadS, err := jsonCompactNoEscape(opts.Payload)
	if err != nil {
		return "", err
	}

	gidS := strings.TrimSpace(opts.GID)

	glimitS := "0"
	if opts.GroupLimit > 0 {
		glimitS = strconv.Itoa(opts.GroupLimit)
	}

	argv := []any{
		jid,
		payloadS,
		strconv.Itoa(opts.MaxAttempts),
		strconv.FormatInt(opts.TimeoutMs, 10),
		strconv.FormatInt(opts.BackoffMs, 10),
		strconv.FormatInt(nms, 10),
		strconv.FormatInt(opts.DueMs, 10),
		gidS,
		glimitS,
	}

	res, err := o.evalShaWithNoScriptFallback(
		o.Scripts.Enqueue.SHA,
		o.Scripts.Enqueue.Src,
		1,
		append([]any{anchor}, argv...)...,
	)
	if err != nil {
		return "", err
	}

	arr, ok := asAnySlice(res)
	if !ok || len(arr) < 2 {
		return "", fmt.Errorf("Unexpected ENQUEUE response: %v", res)
	}

	status := AsStr(arr[0])
	outID := AsStr(arr[1])

	if status != "OK" {
		return "", fmt.Errorf("ENQUEUE failed: %s", status)
	}

	return outID, nil
}

func (o *OmniqOps) Pause(queue string) (string, error) {
	anchor := QueueAnchor(queue)
	res, err := o.evalShaWithNoScriptFallback(
		o.Scripts.Pause.SHA,
		o.Scripts.Pause.Src,
		1,
		anchor,
	)
	if err != nil {
		return "", err
	}
	return AsStr(res), nil
}

func (o *OmniqOps) Resume(queue string) (int, error) {
	anchor := QueueAnchor(queue)
	res, err := o.evalShaWithNoScriptFallback(
		o.Scripts.Resume.SHA,
		o.Scripts.Resume.Src,
		1,
		anchor,
	)
	if err != nil {
		return 0, err
	}
	n, _ := strconv.Atoi(AsStr(res))
	return n, nil
}

func (o *OmniqOps) IsPaused(queue string) (bool, error) {
	base := QueueBase(queue)
	n, err := o.R.Exists(base + ":paused")
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

func (o *OmniqOps) Reserve(queue string, nowMsOverride int64) (ReserveResult, error) {
	anchor := QueueAnchor(queue)

	nms := nowMsOverride
	if nms <= 0 {
		nms = NowMs()
	}

	res, err := o.evalShaWithNoScriptFallback(
		o.Scripts.Reserve.SHA,
		o.Scripts.Reserve.Src,
		1,
		anchor,
		strconv.FormatInt(nms, 10),
	)
	if err != nil {
		return nil, err
	}

	arr, ok := asAnySlice(res)
	if !ok || len(arr) < 1 {
		return nil, fmt.Errorf("Unexpected RESERVE response: %v", res)
	}

	switch AsStr(arr[0]) {
	case "EMPTY":
		return nil, nil
	case "PAUSED":
		return ReservePaused{Status: "PAUSED"}, nil
	case "JOB":
		if len(arr) < 7 {
			return nil, fmt.Errorf("Unexpected RESERVE response: %v", res)
		}

		lockUntil, err := toInt64(arr[3])
		if err != nil {
			return nil, fmt.Errorf("Unexpected RESERVE response: %v", res)
		}
		attempt, err := toInt(arr[4])
		if err != nil {
			return nil, fmt.Errorf("Unexpected RESERVE response: %v", res)
		}

		gid := ""
		if arr[5] != nil {
			gid = AsStr(arr[5])
		}

		lease := ""
		if arr[6] != nil {
			lease = AsStr(arr[6])
		}

		return ReserveJob{
			Status:      "JOB",
			JobID:       AsStr(arr[1]),
			Payload:     AsStr(arr[2]),
			LockUntilMs: lockUntil,
			Attempt:     attempt,
			GID:         gid,
			LeaseToken:  lease,
		}, nil
	default:
		return nil, fmt.Errorf("Unexpected RESERVE response: %v", res)
	}
}

func (o *OmniqOps) Heartbeat(queue, jobID, leaseToken string, nowMsOverride int64) (int64, error) {
	anchor := QueueAnchor(queue)

	nms := nowMsOverride
	if nms <= 0 {
		nms = NowMs()
	}

	res, err := o.evalShaWithNoScriptFallback(
		o.Scripts.Heartbeat.SHA,
		o.Scripts.Heartbeat.Src,
		1,
		anchor,
		jobID,
		strconv.FormatInt(nms, 10),
		leaseToken,
	)
	if err != nil {
		return 0, err
	}

	arr, ok := asAnySlice(res)
	if !ok || len(arr) < 1 {
		return 0, fmt.Errorf("Unexpected HEARTBEAT response: %v", res)
	}

	switch AsStr(arr[0]) {
	case "OK":
		if len(arr) < 2 {
			return 0, fmt.Errorf("Unexpected HEARTBEAT response: %v", res)
		}
		v, err := toInt64(arr[1])
		if err != nil {
			return 0, fmt.Errorf("Unexpected HEARTBEAT response: %v", res)
		}
		return v, nil

	case "ERR":
		reason := "UNKNOWN"
		if len(arr) > 1 {
			reason = AsStr(arr[1])
		}
		return 0, fmt.Errorf("HEARTBEAT failed: %s", reason)

	default:
		return 0, fmt.Errorf("Unexpected HEARTBEAT response: %v", res)
	}
}

func (o *OmniqOps) AckSuccess(queue, jobID, leaseToken string, nowMsOverride int64) error {
	anchor := QueueAnchor(queue)

	nms := nowMsOverride
	if nms <= 0 {
		nms = NowMs()
	}

	res, err := o.evalShaWithNoScriptFallback(
		o.Scripts.AckSuccess.SHA,
		o.Scripts.AckSuccess.Src,
		1,
		anchor,
		jobID,
		strconv.FormatInt(nms, 10),
		leaseToken,
	)
	if err != nil {
		return err
	}

	arr, ok := asAnySlice(res)
	if !ok || len(arr) < 1 {
		return fmt.Errorf("Unexpected ACK_SUCCESS response: %v", res)
	}

	switch AsStr(arr[0]) {
	case "OK":
		return nil

	case "ERR":
		reason := "UNKNOWN"
		if len(arr) > 1 {
			reason = AsStr(arr[1])
		}
		return fmt.Errorf("ACK_SUCCESS failed: %s", reason)

	default:
		return fmt.Errorf("Unexpected ACK_SUCCESS response: %v", res)
	}
}

func (o *OmniqOps) AckFail(queue, jobID, leaseToken string, errMsg *string, nowMsOverride int64) (AckFailResult, error) {
	anchor := QueueAnchor(queue)

	nms := nowMsOverride
	if nms <= 0 {
		nms = NowMs()
	}

	args := []any{
		anchor,
		jobID,
		strconv.FormatInt(nms, 10),
		leaseToken,
	}
	if errMsg != nil && strings.TrimSpace(*errMsg) != "" {
		args = append(args, *errMsg)
	}

	res, err := o.evalShaWithNoScriptFallback(
		o.Scripts.AckFail.SHA,
		o.Scripts.AckFail.Src,
		1,
		args...,
	)
	if err != nil {
		return AckFailResult{}, err
	}

	arr, ok := asAnySlice(res)
	if !ok || len(arr) < 1 {
		return AckFailResult{}, fmt.Errorf("Unexpected ACK_FAIL response: %v", res)
	}

	switch AsStr(arr[0]) {
	case "RETRY":
		if len(arr) < 2 {
			return AckFailResult{}, fmt.Errorf("Unexpected ACK_FAIL response: %v", res)
		}
		n, err := toInt64(arr[1])
		if err != nil {
			return AckFailResult{}, fmt.Errorf("Unexpected ACK_FAIL response: %v", res)
		}
		return AckFailResult{Status: AckRetry, NextRunAtMs: &n}, nil

	case "FAILED":
		return AckFailResult{Status: AckFailed, NextRunAtMs: nil}, nil

	case "ERR":
		reason := "UNKNOWN"
		if len(arr) > 1 {
			reason = AsStr(arr[1])
		}
		return AckFailResult{}, fmt.Errorf("ACK_FAIL failed: %s", reason)

	default:
		return AckFailResult{}, fmt.Errorf("Unexpected ACK_FAIL response: %v", res)
	}
}

func (o *OmniqOps) PromoteDelayed(queue string, maxPromote int, nowMsOverride int64) (int, error) {
	if maxPromote <= 0 {
		maxPromote = 1000
	}

	anchor := QueueAnchor(queue)

	nms := nowMsOverride
	if nms <= 0 {
		nms = NowMs()
	}

	res, err := o.evalShaWithNoScriptFallback(
		o.Scripts.PromoteDelayed.SHA,
		o.Scripts.PromoteDelayed.Src,
		1,
		anchor,
		strconv.FormatInt(nms, 10),
		strconv.Itoa(maxPromote),
	)
	if err != nil {
		return 0, err
	}

	arr, ok := asAnySlice(res)
	if !ok || len(arr) < 2 || AsStr(arr[0]) != "OK" {
		return 0, fmt.Errorf("Unexpected PROMOTE_DELAYED response: %v", res)
	}

	n, err := toInt(arr[1])
	if err != nil {
		return 0, fmt.Errorf("Unexpected PROMOTE_DELAYED response: %v", res)
	}
	return n, nil
}

func (o *OmniqOps) ReapExpired(queue string, maxReap int, nowMsOverride int64) (int, error) {
	if maxReap <= 0 {
		maxReap = 1000
	}

	anchor := QueueAnchor(queue)

	nms := nowMsOverride
	if nms <= 0 {
		nms = NowMs()
	}

	res, err := o.evalShaWithNoScriptFallback(
		o.Scripts.ReapExpired.SHA,
		o.Scripts.ReapExpired.Src,
		1,
		anchor,
		strconv.FormatInt(nms, 10),
		strconv.Itoa(maxReap),
	)
	if err != nil {
		return 0, err
	}

	arr, ok := asAnySlice(res)
	if !ok || len(arr) < 2 || AsStr(arr[0]) != "OK" {
		return 0, fmt.Errorf("Unexpected REAP_EXPIRED response: %v", res)
	}

	n, err := toInt(arr[1])
	if err != nil {
		return 0, fmt.Errorf("Unexpected REAP_EXPIRED response: %v", res)
	}
	return n, nil
}

func (o *OmniqOps) JobTimeoutMs(queue, jobID string, defaultMs int64) (int64, error) {
	if defaultMs <= 0 {
		defaultMs = 60_000
	}

	base := QueueBase(queue)
	kJob := base + ":job:" + jobID

	v, err := o.R.HGet(kJob, "timeout_ms")
	if err != nil {
		return 0, err
	}

	var n int64
	if v != nil && strings.TrimSpace(*v) != "" {
		n, _ = strconv.ParseInt(strings.TrimSpace(*v), 10, 64)
	}

	if n > 0 {
		return n, nil
	}
	return defaultMs, nil
}

func PausedBackoffS(pollIntervalS float64) float64 {
	return math.Max(0.25, pollIntervalS*10.0)
}

func DeriveHeartbeatIntervalS(timeoutMs int64) float64 {
	half := math.Max(1.0, (float64(timeoutMs)/1000.0)/2.0)
	return math.Max(1.0, math.Min(10.0, half))
}
