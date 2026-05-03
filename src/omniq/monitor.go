package omniq

type Monitor struct {
	core *MonitorCore
}

func NewMonitor(client *Client) (*Monitor, error) {
	core, err := NewMonitorCore(client)
	if err != nil {
		return nil, err
	}

	return &Monitor{
		core: core,
	}, nil
}

func (m *Monitor) ScanQueues() []string {
	return m.core.ScanQueues()
}

func (m *Monitor) Stats(queue string) QueueStats {
	return m.core.Stats(queue)
}

func (m *Monitor) StatsMany(queues []string) []QueueStats {
	return m.core.StatsMany(queues)
}

func (m *Monitor) GroupsReady(
	queue string,
	offset int,
	limit int,
) []string {
	return m.core.GroupsReady(queue, offset, limit)
}

func (m *Monitor) GroupsReadyWithScores(
	queue string,
	offset int,
	limit int,
) []GroupReady {
	return m.core.GroupsReadyWithScores(queue, offset, limit)
}

func (m *Monitor) GroupStatus(
	queue string,
	gids []string,
	defaultLimit int,
) []GroupStatus {
	return m.core.GroupStatus(queue, gids, defaultLimit)
}

func (m *Monitor) LanePage(
	queue string,
	lane LaneName,
	offset int,
	limit int,
	reverse bool,
) []LaneJob {
	return m.core.LanePage(queue, lane, offset, limit, reverse)
}

func (m *Monitor) GetJob(queue string, jobID string) *JobInfo {
	return m.core.GetJob(queue, jobID)
}

func (m *Monitor) FindJobs(
	queue string,
	lane LaneName,
	jobIDs []string,
) []LaneJob {
	return m.core.FindJobs(queue, lane, jobIDs)
}

func (m *Monitor) Overview(queue string, samplesPerLane int) QueueOverview {
	return m.core.Overview(queue, samplesPerLane)
}
