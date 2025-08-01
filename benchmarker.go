package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Benchmarker orchestrates the entire benchmarking process
type Benchmarker struct {
	config  Config
	client  *Client
	metrics *Metrics
	display *Display
}

// BenchmarkResult holds the final benchmark results
type BenchmarkResult struct {
	Model             string        `json:"model"`
	Duration          time.Duration `json:"duration"`
	TotalRequests     int64         `json:"total_requests"`
	SuccessfulReqs    int64         `json:"successful_requests"`
	FailedReqs        int64         `json:"failed_requests"`
	RequestsPerSec    float64       `json:"requests_per_second"`
	TokensPerSec      float64       `json:"tokens_per_second"`
	InputTokensPerSec float64       `json:"input_tokens_per_second"`
	OutputTokensPerSec float64      `json:"output_tokens_per_second"`
	AvgLatency        time.Duration `json:"avg_latency"`
	MinLatency        time.Duration `json:"min_latency"`
	MaxLatency        time.Duration `json:"max_latency"`
	P95Latency        time.Duration `json:"p95_latency"`
	P99Latency        time.Duration `json:"p99_latency"`
	AvgTTFT           time.Duration `json:"avg_ttft"`
	ErrorRate         float64       `json:"error_rate"`
	Errors            map[string]int `json:"errors"`
}

// NewBenchmarker creates a new benchmarker instance
func NewBenchmarker(config Config) *Benchmarker {
	return &Benchmarker{
		config:  config,
		client:  NewClient(config.ServerURL, config.Timeout, config.APIKey),
		metrics: NewMetrics(),
		display: NewDisplay(config.NoColor),
	}
}

// Run executes the complete benchmark
func (b *Benchmarker) Run(ctx context.Context) error {
	var modelsToTest []string
	
	// If models are specified, use them directly (no discovery needed)
	if len(b.config.Models) > 0 {
		modelsToTest = b.config.Models
		b.display.PrintStatus(fmt.Sprintf("Using specified models: %s", strings.Join(modelsToTest, ", ")))
	} else if b.config.SkipDiscovery {
		return fmt.Errorf("--skip-discovery requires --models to be specified")
	} else {
		// Discover available models only when no models specified
		if err := b.discoverModels(ctx); err != nil {
			return fmt.Errorf("failed to discover models: %w", err)
		}
		
		// Use all discovered models
		modelsToTest = b.getModelsToTest()
		if len(modelsToTest) == 0 {
			return fmt.Errorf("no models available for testing")
		}
	}
	
	b.display.PrintHeader(b.config.ServerURL, modelsToTest)
	
	var results []BenchmarkResult
	
	// Benchmark each model
	for _, model := range modelsToTest {
		if ctx.Err() != nil {
			break
		}
		
		result, err := b.benchmarkModel(ctx, model)
		if err != nil {
			b.display.PrintError(fmt.Sprintf("Failed to benchmark model %s: %v", model, err))
			continue
		}
		
		results = append(results, result)
	}
	
	// Display final results
	b.display.PrintResults(results, b.config.Format)
	
	return nil
}

// discoverModels fetches available models from the server
func (b *Benchmarker) discoverModels(ctx context.Context) error {
	b.display.PrintStatus("Discovering available models...")
	
	models, err := b.client.ListModels(ctx)
	if err != nil {
		return err
	}
	
	b.client.availableModels = models
	b.display.PrintStatus(fmt.Sprintf("Found %d models", len(models)))
	
	return nil
}

// getModelsToTest determines which models to benchmark based on config
func (b *Benchmarker) getModelsToTest() []string {
	if len(b.config.Models) > 0 {
		// Filter requested models against available models
		var validModels []string
		for _, requested := range b.config.Models {
			for _, available := range b.client.availableModels {
				if available.ID == requested {
					validModels = append(validModels, requested)
					break
				}
			}
		}
		return validModels
	}
	
	// Use all available models
	var models []string
	for _, model := range b.client.availableModels {
		models = append(models, model.ID)
	}
	return models
}

// benchmarkModel runs the complete benchmark for a single model
func (b *Benchmarker) benchmarkModel(ctx context.Context, model string) (BenchmarkResult, error) {
	b.display.PrintModelHeader(model)
	
	// Reset metrics for this model
	b.metrics.Reset()
	
	// Determine optimal concurrency if not specified
	concurrency := b.config.Concurrency
	if concurrency == 0 {
		var err error
		concurrency, err = b.findOptimalConcurrency(ctx, model)
		if err != nil {
			return BenchmarkResult{}, err
		}
	} else if concurrency == -1 {
		// Force concurrency of 1 (good for vLLM systems)
		concurrency = 1
		b.display.PrintStatus("Forced concurrency: 1 (optimal for vLLM systems)")
	} else if concurrency < 0 {
		concurrency = 1
	}
	
	// Warmup phase
	if b.config.Warmup > 0 {
		if err := b.warmup(ctx, model, concurrency); err != nil {
			b.display.PrintError(fmt.Sprintf("Warmup failed: %v", err))
		}
	}
	
	// Main benchmark
	result, err := b.runMainBenchmark(ctx, model, concurrency)
	if err != nil {
		return BenchmarkResult{}, err
	}
	
	return result, nil
}

// findOptimalConcurrency determines the best concurrency level
func (b *Benchmarker) findOptimalConcurrency(ctx context.Context, model string) (int, error) {
	b.display.PrintStatus("Using concurrency: 1 (default for cloud APIs)")
	
	// For cloud APIs, just use concurrency 1 without any testing
	// This eliminates the extra failing request that was coming from the test
	return 1, nil
}

// testConcurrency runs a short test at a specific concurrency level
func (b *Benchmarker) testConcurrency(ctx context.Context, model string, concurrency int, duration time.Duration) (float64, error) {
	tempMetrics := NewMetrics()
	
	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.worker(ctx, model, tempMetrics)
		}()
	}
	
	wg.Wait()
	
	stats := tempMetrics.GetStats()
	return stats.RequestsPerSec, nil
}

// warmup runs the warmup phase
func (b *Benchmarker) warmup(ctx context.Context, model string, concurrency int) error {
	b.display.PrintStatus(fmt.Sprintf("Warming up (%v)...", b.config.Warmup))
	
	ctx, cancel := context.WithTimeout(ctx, b.config.Warmup)
	defer cancel()
	
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Use a throwaway metrics instance for warmup
			warmupMetrics := NewMetrics()
			b.worker(ctx, model, warmupMetrics)
		}()
	}
	
	wg.Wait()
	return nil
}

// runMainBenchmark executes the main benchmark phase
func (b *Benchmarker) runMainBenchmark(ctx context.Context, model string, concurrency int) (BenchmarkResult, error) {
	b.display.PrintStatus(fmt.Sprintf("Running benchmark (concurrency: %d, duration: %v)...", 
		concurrency, b.config.Duration))
	
	// Start progress display in a separate goroutine
	progressCtx, progressCancel := context.WithCancel(ctx)
	progressDone := make(chan struct{})
	
	go func() {
		b.display.ShowProgress(progressCtx, b.metrics, b.config.Duration)
		close(progressDone)
	}()
	
	// Run benchmark workers
	benchCtx, benchCancel := context.WithTimeout(ctx, b.config.Duration)
	defer benchCancel()
	
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.worker(benchCtx, model, b.metrics)
		}()
	}
	
	wg.Wait()
	
	// Cancel progress context and wait for it to finish
	progressCancel()
	<-progressDone
	
	// Get final stats
	stats := b.metrics.GetStats()
	
	return BenchmarkResult{
		Model:              model,
		Duration:           stats.Duration,
		TotalRequests:      stats.TotalRequests,
		SuccessfulReqs:     stats.SuccessfulRequests,
		FailedReqs:         stats.FailedRequests,
		RequestsPerSec:     stats.RequestsPerSec,
		TokensPerSec:       stats.TokensPerSec,
		InputTokensPerSec:  stats.InputTokensPerSec,
		OutputTokensPerSec: stats.OutputTokensPerSec,
		AvgLatency:         stats.AvgLatency,
		MinLatency:         stats.MinLatency,
		MaxLatency:         stats.MaxLatency,
		P95Latency:         stats.P95Latency,
		P99Latency:         stats.P99Latency,
		AvgTTFT:           stats.AvgTTFT,
		ErrorRate:         stats.ErrorRate,
		Errors:            stats.Errors,
	}, nil
}

// worker runs continuous requests until context is cancelled
func (b *Benchmarker) worker(ctx context.Context, model string, metrics *Metrics) {
	prompts := b.getPrompts()
	promptIndex := 0
	
	for {
		select {
		case <-ctx.Done():
			return
		default:
			prompt := prompts[promptIndex%len(prompts)]
			promptIndex++
			
			var resp *CompletionResponse
			var err error
			
			if b.config.Streaming {
				resp, err = b.client.CreateStreamingCompletion(ctx, model, prompt)
			} else {
				resp, err = b.client.CreateCompletion(ctx, model, prompt)
			}
			
			metrics.RecordRequest(resp, err)
		}
	}
}

// getPrompts returns test prompts based on configured size
func (b *Benchmarker) getPrompts() []string {
	prompts := make(map[string][]string)
	
	prompts["small"] = []string{
		"Hello, world!",
		"What is 2+2?",
		"Tell me a joke.",
		"How are you?",
		"What's the weather like?",
	}
	
	prompts["medium"] = []string{
		"Write a short story about a robot learning to paint.",
		"Explain the concept of recursion in programming with an example.",
		"What are the main differences between renewable and non-renewable energy sources?",
		"Describe the process of photosynthesis in plants.",
		"How does machine learning differ from traditional programming approaches?",
	}
	
	prompts["large"] = []string{
		"You are a senior software engineer reviewing a pull request. The code implements a distributed cache system using Redis. Please provide a comprehensive code review covering architecture, performance, security, error handling, testing, and maintainability. Consider scalability concerns and suggest improvements for monitoring and observability. The system needs to handle 100,000 requests per second with sub-millisecond latency requirements.",
		"Write a detailed technical specification for a real-time collaborative document editing system similar to Google Docs. Include the architecture design, data structures, conflict resolution algorithms, network protocols, security considerations, user authentication, permission management, and scalability strategies. Explain how you would handle concurrent edits, maintain consistency across multiple clients, and ensure data persistence.",
		"Design a comprehensive monitoring and alerting system for a microservices architecture running on Kubernetes. The system should handle metrics collection, log aggregation, distributed tracing, anomaly detection, and automated incident response. Explain the technology stack, data flow, storage requirements, query optimization, dashboard design, and integration with existing DevOps tools.",
	}
	
	switch b.config.PromptSize {
	case "small":
		return prompts["small"]
	case "medium":
		return prompts["medium"]
	case "large":
		return prompts["large"]
	case "all":
		var all []string
		for _, p := range prompts {
			all = append(all, p...)
		}
		return all
	default:
		return prompts["medium"]
	}
}