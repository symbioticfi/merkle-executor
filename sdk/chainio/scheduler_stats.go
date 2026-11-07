package chainio

// SchedulerStats is a value snapshot. Queue sizes include accepted in-flight work,
// and bytes use the scheduler's retained-request/result-reservation budget charge.
// Peaks and dispatch counters are cumulative for this scheduler's lifetime.
type SchedulerStats struct {
	QueuedRequests     int
	QueuedBytes        uint64
	PeakQueuedRequests int
	PeakQueuedBytes    uint64
	// ReadyWaves counts nonempty collections that produced eligible partitions.
	ReadyWaves uint64
	// DispatchedPartitions counts calls to the backend, including failed groups.
	DispatchedPartitions uint64
	// DispatchedUniqueReads counts logical read members passed to the backend;
	// Multicall and HTTP request counts remain backend/transport measurements.
	DispatchedUniqueReads uint64
	// DeduplicatedReadDeliveries counts extra logical result slots sharing one
	// dispatched read. It includes slots whose result is an error or whose owner
	// cancels after admission; entirely abandoned partitions are not dispatched.
	DeduplicatedReadDeliveries uint64
}

// Stats returns an independent, concurrency-safe snapshot without resetting it.
func (s *Scheduler) Stats() SchedulerStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot := s.stats
	snapshot.QueuedRequests = s.count
	snapshot.QueuedBytes = s.bytes
	return snapshot
}
func (s *Scheduler) recordDispatch(partition readPartition) {
	var deliveries uint64
	for _, slots := range partition.slots {
		deliveries += uint64(len(slots) - 1)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.DispatchedPartitions++
	s.stats.DispatchedUniqueReads += uint64(len(partition.requests))
	s.stats.DeduplicatedReadDeliveries += deliveries
}
