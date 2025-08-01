package main

import (
	"sort"
	"sync"
	"time"
)

// Metrics collects and calculates benchmark statistics
type Metrics struct {
	mu                  sync.RWMutex
	startTime           time.Time
	requestTimes        []time.Duration
	ttftTimes           []time.Duration
	inputTokens         []int
	outputTokens        []int
	totalRequests       int64
	successfulRequests  int64
	failedRequests      int64
	errors              map[string]int
}

// MetricsStats represents calculated statistics
type MetricsStats struct {
	Duration           time.Duration
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	RequestsPerSec     float64
	TokensPerSec       float64
	InputTokensPerSec  float64
	OutputTokensPerSec float64
	AvgLatency         time.Duration
	MinLatency         time.Duration
	MaxLatency         time.Duration
	P95Latency         time.Duration
	P99Latency         time.Duration
	AvgTTFT           time.Duration
	ErrorRate         float64
	Errors            map[string]int
}

// NewMetrics creates a new metrics collector
func NewMetrics() *Metrics {
	return &Metrics{
		startTime: time.Now(),
		errors:    make(map[string]int),
	}
}

// Reset clears all metrics and restarts timing
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.startTime = time.Now()
	m.requestTimes = nil
	m.ttftTimes = nil
	m.inputTokens = nil
	m.outputTokens = nil
	m.totalRequests = 0
	m.successfulRequests = 0
	m.failedRequests = 0
	m.errors = make(map[string]int)
}

// RecordRequest records the result of a single request
func (m *Metrics) RecordRequest(resp *CompletionResponse, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.totalRequests++
	
	if err != nil {
		m.failedRequests++
		errMsg := err.Error()
		// Truncate very long error messages
		if len(errMsg) > 100 {
			errMsg = errMsg[:100] + "..."
		}
		m.errors[errMsg]++
		return
	}
	
	if resp == nil {
		m.failedRequests++
		m.errors["nil response"]++
		return
	}
	
	m.successfulRequests++
	
	// Record latency
	latency := resp.ResponseTime.Sub(resp.RequestTime)
	m.requestTimes = append(m.requestTimes, latency)
	
	// Record TTFT for streaming requests
	if resp.TTFT > 0 {
		m.ttftTimes = append(m.ttftTimes, resp.TTFT)
	}
	
	// Record token counts
	m.inputTokens = append(m.inputTokens, resp.Usage.PromptTokens)
	m.outputTokens = append(m.outputTokens, resp.Usage.CompletionTokens)
}

// GetStats calculates and returns current statistics
func (m *Metrics) GetStats() MetricsStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	duration := time.Since(m.startTime)
	
	stats := MetricsStats{
		Duration:           duration,
		TotalRequests:      m.totalRequests,
		SuccessfulRequests: m.successfulRequests,
		FailedRequests:     m.failedRequests,
		Errors:            make(map[string]int),
	}
	
	// Copy errors map
	for k, v := range m.errors {
		stats.Errors[k] = v
	}
	
	// Calculate error rate
	if m.totalRequests > 0 {
		stats.ErrorRate = float64(m.failedRequests) / float64(m.totalRequests) * 100
	}
	
	// Calculate RPS
	if duration.Seconds() > 0 {
		stats.RequestsPerSec = float64(m.successfulRequests) / duration.Seconds()
	}
	
	// Calculate token rates and latency stats
	if len(m.requestTimes) > 0 {
		stats.AvgLatency = m.calculateAverage(m.requestTimes)
		stats.MinLatency = m.calculateMin(m.requestTimes)
		stats.MaxLatency = m.calculateMax(m.requestTimes)
		stats.P95Latency = m.calculatePercentile(m.requestTimes, 95)
		stats.P99Latency = m.calculatePercentile(m.requestTimes, 99)
	}
	
	if len(m.ttftTimes) > 0 {
		stats.AvgTTFT = m.calculateAverage(m.ttftTimes)
	}
	
	// Calculate token rates
	if duration.Seconds() > 0 {
		totalInputTokens := m.sumInts(m.inputTokens)
		totalOutputTokens := m.sumInts(m.outputTokens)
		
		stats.InputTokensPerSec = float64(totalInputTokens) / duration.Seconds()
		stats.OutputTokensPerSec = float64(totalOutputTokens) / duration.Seconds()
		stats.TokensPerSec = stats.InputTokensPerSec + stats.OutputTokensPerSec
	}
	
	return stats
}

// GetCurrentRPS returns the current requests per second
func (m *Metrics) GetCurrentRPS() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	duration := time.Since(m.startTime)
	if duration.Seconds() <= 0 {
		return 0
	}
	
	return float64(m.successfulRequests) / duration.Seconds()
}

// GetCurrentTokensPerSec returns the current tokens per second
func (m *Metrics) GetCurrentTokensPerSec() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	duration := time.Since(m.startTime)
	if duration.Seconds() <= 0 {
		return 0
	}
	
	totalTokens := m.sumInts(m.inputTokens) + m.sumInts(m.outputTokens)
	return float64(totalTokens) / duration.Seconds()
}

// Helper functions

func (m *Metrics) calculateAverage(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	
	var total int64
	for _, d := range durations {
		total += int64(d)
	}
	
	return time.Duration(total / int64(len(durations)))
}

func (m *Metrics) calculateMin(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	
	min := durations[0]
	for _, d := range durations[1:] {
		if d < min {
			min = d
		}
	}
	
	return min
}

func (m *Metrics) calculateMax(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	
	max := durations[0]
	for _, d := range durations[1:] {
		if d > max {
			max = d
		}
	}
	
	return max
}

func (m *Metrics) calculatePercentile(durations []time.Duration, percentile int) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	
	// Make a copy and sort it
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	
	index := int(float64(len(sorted)) * float64(percentile) / 100.0)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	
	return sorted[index]
}

func (m *Metrics) sumInts(values []int) int {
	sum := 0
	for _, v := range values {
		sum += v
	}
	return sum
}

// LiveMetrics provides thread-safe access to current metrics for display
type LiveMetrics struct {
	metrics *Metrics
	lastReqs int64
	lastTokens int
	lastTime time.Time
	recentRPS float64
	recentTPS float64
}

// NewLiveMetrics creates a wrapper for live metrics access
func NewLiveMetrics(metrics *Metrics) *LiveMetrics {
	return &LiveMetrics{
		metrics: metrics,
		lastTime: time.Now(),
	}
}

// GetLiveStats returns current stats without blocking the metrics collector
func (lm *LiveMetrics) GetLiveStats() (float64, float64, int64, int64, time.Duration) {
	lm.metrics.mu.RLock()
	defer lm.metrics.mu.RUnlock()
	
	duration := time.Since(lm.metrics.startTime)
	rps := 0.0
	tps := 0.0
	
	if duration.Seconds() > 0 {
		rps = float64(lm.metrics.successfulRequests) / duration.Seconds()
		totalTokens := lm.metrics.sumInts(lm.metrics.inputTokens) + lm.metrics.sumInts(lm.metrics.outputTokens)
		tps = float64(totalTokens) / duration.Seconds()
	}
	
	// Calculate recent rates (last 5 seconds)
	now := time.Now()
	if now.Sub(lm.lastTime) >= 1*time.Second {
		currentReqs := lm.metrics.successfulRequests
		currentTokens := lm.metrics.sumInts(lm.metrics.inputTokens) + lm.metrics.sumInts(lm.metrics.outputTokens)
		timeDiff := now.Sub(lm.lastTime).Seconds()
		
		if timeDiff > 0 {
			lm.recentRPS = float64(currentReqs-lm.lastReqs) / timeDiff
			lm.recentTPS = float64(currentTokens-lm.lastTokens) / timeDiff
		}
		
		lm.lastReqs = currentReqs
		lm.lastTokens = currentTokens
		lm.lastTime = now
	}
	
	// Use recent rates if they're available and make sense
	displayRPS := rps
	displayTPS := tps
	if lm.recentRPS > 0 && duration.Seconds() > 3 {
		displayRPS = lm.recentRPS
	}
	if lm.recentTPS > 0 && duration.Seconds() > 3 {
		displayTPS = lm.recentTPS
	}
	
	return displayRPS, displayTPS, lm.metrics.successfulRequests, lm.metrics.totalRequests, duration
}