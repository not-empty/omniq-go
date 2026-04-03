package omniq

import (
	"errors"
	"sort"
	"strconv"
	"strings"
)

type MonitorZItem struct {
	Member string
	Score  float64
}

type monitorRedis interface {
	Exists(key string) (int64, error)
	Get(key string) (*string, error)
	LLen(key string) (int64, error)
	SMembers(key string) ([]string, error)
	HGetAll(key string) (map[string]string, error)
	ZScore(key, member string) (*float64, error)
	ZRangeWithScores(key string, start, end int64) ([]MonitorZItem, error)
	ZRevRangeWithScores(key string, start, end int64) ([]MonitorZItem, error)
}

type monitorRedisWrap struct {
	r RedisLike
}

func (w *monitorRedisWrap) Exists(key string) (int64, error) {
	return w.r.Exists(key)
}

func (w *monitorRedisWrap) Get(key string) (*string, error) {
	return w.r.Get(key)
}

func (w *monitorRedisWrap) LLen(key string) (int64, error) {
	return w.r.LLen(key)
}

func (w *monitorRedisWrap) SMembers(key string) ([]string, error) {
	return w.r.SMembers(key)
}

func (w *monitorRedisWrap) HGetAll(key string) (map[string]string, error) {
	return w.r.HGetAll(key)
}

func (w *monitorRedisWrap) ZScore(key, member string) (*float64, error) {
	return w.r.ZScore(key, member)
}

func (w *monitorRedisWrap) ZRangeWithScores(key string, start, end int64) ([]MonitorZItem, error) {
	return w.r.ZRangeWithScores(key, start, end)
}

func (w *monitorRedisWrap) ZRevRangeWithScores(key string, start, end int64) ([]MonitorZItem, error) {
	return w.r.ZRevRangeWithScores(key, start, end)
}

type MonitorCore struct {
	r monitorRedis
}

const (
	monitorQueueRegistry = "omniq:queues"
	monitorMaxListLimit  = 25
	monitorMaxGroupLimit = 500
)

func NewMonitorCore(client *Client) (*MonitorCore, error) {
	if client == nil {
		return nil, errors.New("monitor requires client")
	}

	ops := client.Ops()
	if ops == nil || ops.R == nil {
		return nil, errors.New("monitor requires redis access")
	}

	return &MonitorCore{
		r: &monitorRedisWrap{r: ops.R},
	}, nil
}

func (m *MonitorCore) base(queue string) string {
	return QueueBase(queue)
}

func (m *MonitorCore) statsKey(base string) string {
	return base + ":stats"
}

func (m *MonitorCore) pausedKey(base string) string {
	return base + ":paused"
}

func (m *MonitorCore) readyKey(base string) string {
	return base + ":groups:ready"
}

func (m *MonitorCore) jobKey(base string, jobID string) string {
	return base + ":job:" + jobID
}

func (m *MonitorCore) idxKey(base string, lane LaneName) string {
	return base + ":idx:" + string(lane)
}

func (m *MonitorCore) gwaitKey(base string, gid string) string {
	return base + ":g:" + gid + ":wait"
}

func (m *MonitorCore) ginflightKey(base string, gid string) string {
	return base + ":g:" + gid + ":inflight"
}

func (m *MonitorCore) glimitKey(base string, gid string) string {
	return base + ":g:" + gid + ":limit"
}

func monitorAsString(v any) string {
	if v == nil {
		return ""
	}

	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return ""
	}
}

func monitorToInt64(v any, def int64) int64 {
	s := monitorAsString(v)
	if s == "" {
		return def
	}

	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}

	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int64(f)
	}

	return def
}

func monitorNormalizeQueueName(baseOrQueue string) string {
	value := strings.TrimSpace(baseOrQueue)
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") && len(value) >= 2 {
		return value[1 : len(value)-1]
	}
	return value
}

func monitorClamp(limit, minValue, maxValue int) int {
	if limit < minValue {
		return minValue
	}
	if limit > maxValue {
		return maxValue
	}
	return limit
}

func monitorMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m *MonitorCore) readJobMap(base string, jobID string) map[string]string {
	key := m.jobKey(base, jobID)

	exists, err := m.r.Exists(key)
	if err != nil || exists != 1 {
		return nil
	}

	raw, err := m.r.HGetAll(key)
	if err != nil || len(raw) == 0 {
		return nil
	}

	return raw
}

func (m *MonitorCore) jobInfoFromMap(jobID string, mm map[string]string) JobInfo {
	return JobInfo{
		JobID:          jobID,
		State:          mm["state"],
		GID:            mm["gid"],
		Attempt:        monitorToInt64(mm["attempt"], 0),
		MaxAttempts:    monitorToInt64(mm["max_attempts"], 0),
		TimeoutMS:      monitorToInt64(mm["timeout_ms"], 0),
		BackoffMS:      monitorToInt64(mm["backoff_ms"], 0),
		LeaseToken:     mm["lease_token"],
		LockUntilMS:    monitorToInt64(mm["lock_until_ms"], 0),
		DueMS:          monitorToInt64(mm["due_ms"], 0),
		Payload:        mm["payload"],
		LastError:      mm["last_error"],
		LastErrorMS:    monitorToInt64(mm["last_error_ms"], 0),
		CreatedMS:      monitorToInt64(mm["created_ms"], 0),
		UpdatedMS:      monitorToInt64(mm["updated_ms"], 0),
		QueuedMS:       monitorToInt64(mm["queued_ms"], 0),
		FirstStartedMS: monitorToInt64(mm["first_started_ms"], 0),
		LastStartedMS:  monitorToInt64(mm["last_started_ms"], 0),
		CompletedMS:    monitorToInt64(mm["completed_ms"], 0),
		FailedMS:       monitorToInt64(mm["failed_ms"], 0),
	}
}

func (m *MonitorCore) laneJobFromMap(
	lane LaneName,
	jobID string,
	idxScoreMS int64,
	mm map[string]string,
) LaneJob {
	return LaneJob{
		Lane:           lane,
		JobID:          jobID,
		IdxScoreMS:     idxScoreMS,
		State:          mm["state"],
		GID:            mm["gid"],
		Attempt:        monitorToInt64(mm["attempt"], 0),
		MaxAttempts:    monitorToInt64(mm["max_attempts"], 0),
		DueMS:          monitorToInt64(mm["due_ms"], 0),
		LockUntilMS:    monitorToInt64(mm["lock_until_ms"], 0),
		QueuedMS:       monitorToInt64(mm["queued_ms"], 0),
		FirstStartedMS: monitorToInt64(mm["first_started_ms"], 0),
		LastStartedMS:  monitorToInt64(mm["last_started_ms"], 0),
		CompletedMS:    monitorToInt64(mm["completed_ms"], 0),
		FailedMS:       monitorToInt64(mm["failed_ms"], 0),
		UpdatedMS:      monitorToInt64(mm["updated_ms"], 0),
		LastError:      mm["last_error"],
	}
}

func (m *MonitorCore) ListQueues() []string {
	bases, err := m.r.SMembers(monitorQueueRegistry)
	if err != nil {
		return []string{}
	}

	out := make([]string, 0, len(bases))
	for _, x := range bases {
		s := strings.TrimSpace(x)
		if s == "" {
			continue
		}
		out = append(out, monitorNormalizeQueueName(s))
	}

	sort.Strings(out)
	return out
}

func (m *MonitorCore) Stats(queue string) QueueStats {
	base := m.base(queue)

	raw, err := m.r.HGetAll(m.statsKey(base))
	if err != nil {
		raw = map[string]string{}
	}

	paused := false
	if exists, err := m.r.Exists(m.pausedKey(base)); err == nil && exists == 1 {
		paused = true
	}

	waiting := monitorToInt64(raw["waiting"], 0)
	groupWaiting := monitorToInt64(raw["group_waiting"], 0)
	waitingTotal := monitorToInt64(raw["waiting_total"], 0)

	if waitingTotal <= 0 && (waiting > 0 || groupWaiting > 0) {
		waitingTotal = waiting + groupWaiting
	}

	return QueueStats{
		Queue:          monitorNormalizeQueueName(queue),
		Paused:         paused,
		Waiting:        waiting,
		GroupWaiting:   groupWaiting,
		WaitingTotal:   waitingTotal,
		Active:         monitorToInt64(raw["active"], 0),
		Delayed:        monitorToInt64(raw["delayed"], 0),
		Failed:         monitorToInt64(raw["failed"], 0),
		CompletedKept:  monitorToInt64(raw["completed_kept"], 0),
		GroupsReady:    monitorToInt64(raw["groups_ready"], 0),
		LastActivityMS: monitorToInt64(raw["last_activity_ms"], 0),
		LastEnqueueMS:  monitorToInt64(raw["last_enqueue_ms"], 0),
		LastReserveMS:  monitorToInt64(raw["last_reserve_ms"], 0),
		LastFinishMS:   monitorToInt64(raw["last_finish_ms"], 0),
	}
}

func (m *MonitorCore) StatsMany(queues []string) []QueueStats {
	target := queues
	if target == nil {
		target = m.ListQueues()
	}

	out := make([]QueueStats, 0, len(target))
	for _, q := range target {
		out = append(out, m.Stats(q))
	}

	return out
}

func (m *MonitorCore) GroupsReady(queue string, offset int, limit int) []string {
	rows := m.GroupsReadyWithScores(queue, offset, limit)

	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.GID == "" {
			continue
		}
		out = append(out, row.GID)
	}

	return out
}

func (m *MonitorCore) GroupsReadyWithScores(queue string, offset int, limit int) []GroupReady {
	base := m.base(queue)
	offset = monitorMax(0, offset)
	limit = monitorClamp(limit, 1, monitorMaxGroupLimit)

	rows, err := m.r.ZRangeWithScores(
		m.readyKey(base),
		int64(offset),
		int64(offset+limit-1),
	)
	if err != nil {
		return []GroupReady{}
	}

	out := make([]GroupReady, 0, len(rows))
	for _, row := range rows {
		gid := strings.TrimSpace(row.Member)
		if gid == "" {
			continue
		}

		out = append(out, GroupReady{
			GID:     gid,
			ScoreMS: int64(row.Score),
		})
	}

	return out
}

func (m *MonitorCore) GroupStatus(queue string, gids []string, defaultLimit int) []GroupStatus {
	base := m.base(queue)
	if defaultLimit < 1 {
		defaultLimit = 1
	}

	normalized := make([]string, 0, len(gids))
	for _, gid := range gids {
		gid = strings.TrimSpace(gid)
		if gid != "" {
			normalized = append(normalized, gid)
		}
	}
	if len(normalized) > monitorMaxGroupLimit {
		normalized = normalized[:monitorMaxGroupLimit]
	}

	readyIDs := m.GroupsReady(queue, 0, monitorMax(len(normalized), 1))
	readySet := make(map[string]struct{}, len(readyIDs))
	for _, gid := range readyIDs {
		readySet[gid] = struct{}{}
	}

	out := make([]GroupStatus, 0, len(normalized))
	for _, gid := range normalized {
		inflightRaw, err := m.r.Get(m.ginflightKey(base, gid))
		if err != nil {
			inflightRaw = nil
		}

		limitRaw, err := m.r.Get(m.glimitKey(base, gid))
		if err != nil {
			limitRaw = nil
		}

		waitingCount, err := m.r.LLen(m.gwaitKey(base, gid))
		if err != nil {
			waitingCount = 0
		}

		limitValue := monitorToInt64(limitRaw, 0)
		if limitValue <= 0 {
			limitValue = int64(defaultLimit)
		}

		_, ready := readySet[gid]

		out = append(out, GroupStatus{
			GID:          gid,
			Inflight:     monitorToInt64(inflightRaw, 0),
			Limit:        limitValue,
			Ready:        ready,
			WaitingCount: waitingCount,
		})
	}

	return out
}

func (m *MonitorCore) LanePage(
	queue string,
	lane LaneName,
	offset int,
	limit int,
	reverse bool,
) []LaneJob {
	base := m.base(queue)
	offset = monitorMax(0, offset)
	limit = monitorClamp(limit, 1, monitorMaxListLimit)
	key := m.idxKey(base, lane)

	var (
		rows []MonitorZItem
		err  error
	)

	if reverse {
		rows, err = m.r.ZRevRangeWithScores(
			key,
			int64(offset),
			int64(offset+limit-1),
		)
	} else {
		rows, err = m.r.ZRangeWithScores(
			key,
			int64(offset),
			int64(offset+limit-1),
		)
	}

	if err != nil {
		return []LaneJob{}
	}

	out := make([]LaneJob, 0, len(rows))
	for _, row := range rows {
		jobID := strings.TrimSpace(row.Member)
		if jobID == "" {
			continue
		}

		mm := m.readJobMap(base, jobID)
		if mm == nil {
			continue
		}

		out = append(out, m.laneJobFromMap(
			lane,
			jobID,
			int64(row.Score),
			mm,
		))
	}

	return out
}

func (m *MonitorCore) GetJob(queue string, jobID string) *JobInfo {
	base := m.base(queue)
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return nil
	}

	mm := m.readJobMap(base, jobID)
	if mm == nil {
		return nil
	}

	job := m.jobInfoFromMap(jobID, mm)
	return &job
}

func (m *MonitorCore) FindJobs(queue string, lane LaneName, jobIDs []string) []LaneJob {
	base := m.base(queue)
	idxKey := m.idxKey(base, lane)

	out := make([]LaneJob, 0, len(jobIDs))
	for _, rawJobID := range jobIDs {
		jobID := strings.TrimSpace(rawJobID)
		if jobID == "" {
			continue
		}

		score, err := m.r.ZScore(idxKey, jobID)
		if err != nil || score == nil {
			continue
		}

		mm := m.readJobMap(base, jobID)
		if mm == nil {
			continue
		}

		out = append(out, m.laneJobFromMap(
			lane,
			jobID,
			int64(*score),
			mm,
		))
	}

	return out
}

func (m *MonitorCore) Overview(queue string, samplesPerLane int) QueueOverview {
	samplesPerLane = monitorClamp(samplesPerLane, 1, monitorMaxListLimit)

	return QueueOverview{
		Stats:       m.Stats(queue),
		ReadyGroups: m.GroupsReadyWithScores(queue, 0, samplesPerLane),
		Active:      m.LanePage(queue, LaneActive, 0, samplesPerLane, false),
		Delayed:     m.LanePage(queue, LaneDelayed, 0, samplesPerLane, false),
		Failed:      m.LanePage(queue, LaneFailed, 0, samplesPerLane, false),
		Completed:   m.LanePage(queue, LaneCompleted, 0, samplesPerLane, false),
	}
}