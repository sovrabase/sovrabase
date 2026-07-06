package functions

import (
	"runtime"
	"sync"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/functions/procstats"
)

const (
	sampleInterval = 100 * time.Millisecond
	maxSamples     = 60
)

// MetricSample is a point-in-time measurement during execution.
type MetricSample struct {
	OffsetMs  int64 `json:"offset_ms"`
	HeapBytes int64 `json:"heap_bytes"`
	CPUMs     int64 `json:"cpu_ms"`
}

// ExecutionMetrics summarizes resource usage for one run.
type ExecutionMetrics struct {
	DurationMs    int64          `json:"duration_ms"`
	MemAllocBytes int64          `json:"mem_alloc_bytes"`
	MemPeakBytes  int64          `json:"mem_peak_bytes"`
	CPUUserMs     int64          `json:"cpu_user_ms"`
	CPUSysMs      int64          `json:"cpu_sys_ms"`
	Samples       []MetricSample `json:"samples,omitempty"`
	Isolated      bool           `json:"isolated,omitempty"` // true when measured from worker subprocess
}

// MetricsCollector samples heap and CPU during a function run.
type MetricsCollector struct {
	start     time.Time
	startHeap uint64
	startCPU  procstats.CPUTimes
	stopCh    chan struct{}
	doneCh    chan struct{}
	mu        sync.Mutex
	samples   []MetricSample
	peakHeap  int64
}

// Start begins background sampling.
func (m *MetricsCollector) Start() *MetricsCollector {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	m.start = time.Now()
	m.startHeap = ms.HeapInuse
	m.startCPU = procstats.ReadCPUTimes()
	m.stopCh = make(chan struct{})
	m.doneCh = make(chan struct{})

	go func() {
		ticker := time.NewTicker(sampleInterval)
		defer ticker.Stop()
		defer close(m.doneCh)
		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.recordSample()
			}
		}
	}()
	return m
}

func (m *MetricsCollector) recordSample() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	cpu := procstats.ReadCPUTimes()
	heap := int64(ms.HeapInuse)
	if heap > m.peakHeap {
		m.peakHeap = heap
	}

	userDelta := cpu.User - m.startCPU.User
	sysDelta := cpu.Sys - m.startCPU.Sys
	cpuMs := (userDelta + sysDelta).Milliseconds()

	m.mu.Lock()
	m.samples = append(m.samples, MetricSample{
		OffsetMs:  time.Since(m.start).Milliseconds(),
		HeapBytes: heap,
		CPUMs:     cpuMs,
	})
	m.mu.Unlock()
}

// Stop ends sampling and returns aggregated metrics.
func (m *MetricsCollector) Stop() ExecutionMetrics {
	close(m.stopCh)
	<-m.doneCh

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	endCPU := procstats.ReadCPUTimes()

	duration := time.Since(m.start)
	userDelta := endCPU.User - m.startCPU.User
	sysDelta := endCPU.Sys - m.startCPU.Sys

	heapEnd := int64(ms.HeapInuse)
	memDelta := heapEnd - int64(m.startHeap)
	if memDelta < 0 {
		memDelta = 0
	}
	if int64(ms.HeapInuse) > m.peakHeap {
		m.peakHeap = int64(ms.HeapInuse)
	}

	m.mu.Lock()
	samples := downsampleSamples(m.samples, maxSamples)
	m.mu.Unlock()

	return ExecutionMetrics{
		DurationMs:    duration.Milliseconds(),
		MemAllocBytes: memDelta,
		MemPeakBytes:  m.peakHeap,
		CPUUserMs:     userDelta.Milliseconds(),
		CPUSysMs:      sysDelta.Milliseconds(),
		Samples:       samples,
	}
}

func downsampleSamples(samples []MetricSample, max int) []MetricSample {
	if len(samples) <= max {
		return samples
	}
	out := make([]MetricSample, 0, max)
	step := float64(len(samples)) / float64(max)
	for i := 0; i < max; i++ {
		idx := int(float64(i) * step)
		if idx >= len(samples) {
			idx = len(samples) - 1
		}
		out = append(out, samples[idx])
	}
	return out
}
