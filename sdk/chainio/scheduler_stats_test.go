package chainio

import (
	"testing"
	"time"
)

func TestSchedulerStatsSnapshotsTrackQueueAndCumulativeDispatch(t *testing.T) {
	gate := make(chan struct{})
	b := &recordingBackend{wait: gate}
	s := startScheduler(t, b, testLimits())
	if got := s.Stats(); got != (SchedulerStats{}) {
		t.Fatalf("new stats: %+v", got)
	}
	monitorDone := make(chan struct{})
	monitorStopped := make(chan struct{})
	go func() {
		defer close(monitorStopped)
		for {
			select {
			case <-monitorDone:
				return
			default:
				_ = s.Stats()
			}
		}
	}()
	defer func() { close(monitorDone); <-monitorStopped }()
	done := make(chan error, 1)
	go func() { _, e := s.BatchRead(t.Context(), []ReadRequest{request(1), request(1)}); done <- e }()
	deadline := time.Now().Add(time.Second)
	for b.active.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("backend did not start")
		}
		time.Sleep(time.Millisecond)
	}
	active := s.Stats()
	if active.QueuedRequests != 2 || active.QueuedBytes == 0 || active.PeakQueuedRequests != 2 || active.PeakQueuedBytes != active.QueuedBytes {
		t.Fatalf("active queue: %+v", active)
	}
	if active.ReadyWaves != 1 || active.DispatchedPartitions != 1 || active.DispatchedUniqueReads != 1 || active.DeduplicatedReadDeliveries != 1 {
		t.Fatalf("active counters: %+v", active)
	}
	close(gate)
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	drained := s.Stats()
	if drained.QueuedRequests != 0 || drained.QueuedBytes != 0 || drained.PeakQueuedBytes != active.PeakQueuedBytes {
		t.Fatalf("drained queue: %+v", drained)
	}
	if active.QueuedRequests != 2 || active.QueuedBytes == 0 {
		t.Fatal("previous snapshot mutated")
	}
	if _, e := s.BatchRead(t.Context(), []ReadRequest{request(2)}); e != nil {
		t.Fatal(e)
	}
	final := s.Stats()
	if final.ReadyWaves != 2 || final.DispatchedPartitions != 2 || final.DispatchedUniqueReads != 2 || final.DeduplicatedReadDeliveries != 1 {
		t.Fatalf("cumulative counters: %+v", final)
	}
}
