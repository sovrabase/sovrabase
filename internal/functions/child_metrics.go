package functions

import (
	"sync"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/functions/procstats"
)

// ChildMetricsCollector samples an isolated worker subprocess by PID.
type ChildMetricsCollector struct {
	pid       int
	start     time.Time
	startCPU  procstats.CPUTimes
	stopCh    chan struct{}
	doneCh    chan struct{}
	mu        sync.Mutex
	samples   []MetricSample
	peakRSS   int64
}

// StartChildMetrics begins sampling the worker process identified by pid.
func StartChildMetrics(pid int) *ChildMetricsCollector {
	c := &ChildMetricsCollector{pid: pid}
	if stats, err := procstats.ReadProcessStats(pid); err == nil {
		c.startCPU = stats.CPU
	}
	c.start = time.Now()
	c.stopCh = make(chan struct{})
	c.doneCh = make(chan struct{})

	go func() {
		ticker := time.NewTicker(sampleInterval)
		defer ticker.Stop()
		defer close(c.doneCh)
		for {
			select {
			case <-c.stopCh:
				return
			case <-ticker.C:
				c.recordSample()
			}
		}
	}()
	return c
}

func (c *ChildMetricsCollector) recordSample() {
	stats, err := procstats.ReadProcessStats(c.pid)
	if err != nil {
		return
	}
	if stats.RSSBytes > c.peakRSS {
		c.peakRSS = stats.RSSBytes
	}
	cpuMs := (stats.CPU.User - c.startCPU.User + stats.CPU.Sys - c.startCPU.Sys).Milliseconds()
	if cpuMs < 0 {
		cpuMs = 0
	}

	c.mu.Lock()
	c.samples = append(c.samples, MetricSample{
		OffsetMs:  time.Since(c.start).Milliseconds(),
		HeapBytes: stats.RSSBytes,
		CPUMs:     cpuMs,
	})
	c.mu.Unlock()
}

// Stop ends sampling and returns metrics for the isolated worker.
func (c *ChildMetricsCollector) Stop(processState interface{}) ExecutionMetrics {
	close(c.stopCh)
	<-c.doneCh

	duration := time.Since(c.start)
	finalCPU := procstats.ChildCPUTimes(processState)

	c.mu.Lock()
	samples := downsampleSamples(c.samples, maxSamples)
	c.mu.Unlock()

	return ExecutionMetrics{
		DurationMs:    duration.Milliseconds(),
		MemAllocBytes: c.peakRSS,
		MemPeakBytes:  c.peakRSS,
		CPUUserMs:     finalCPU.User.Milliseconds(),
		CPUSysMs:      finalCPU.Sys.Milliseconds(),
		Samples:       samples,
		Isolated:      true,
	}
}
