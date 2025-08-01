package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// Config holds all benchmark configuration
type Config struct {
	ServerURL    string
	Duration     time.Duration
	Concurrency  int
	Models       []string
	Timeout      time.Duration
	Warmup       time.Duration
	Streaming    bool
	PromptSize   string
	Format       string
	Verbose      bool
	NoColor      bool
	APIKey       string
	SkipDiscovery bool
}

var config Config

var rootCmd = &cobra.Command{
	Use:   "aibench [flags] <server-url>",
	Short: "Benchmark OpenAI-compatible API servers",
	Long: `aibench is a CLI tool for benchmarking OpenAI-compatible API servers.
It automatically discovers available models and measures performance metrics
like requests per second, tokens per second, and response latency.`,
	Args: cobra.ExactArgs(1),
	RunE: runBenchmark,
}

func init() {
	rootCmd.Flags().DurationVarP(&config.Duration, "duration", "d", 30*time.Second, "Duration to run benchmark")
	rootCmd.Flags().IntVarP(&config.Concurrency, "concurrency", "c", 0, "Number of concurrent requests (0 = auto-detect, -1 = force 1)")
	rootCmd.Flags().StringSliceVarP(&config.Models, "models", "m", nil, "Comma-separated models to test (default: all discovered)")
	rootCmd.Flags().DurationVarP(&config.Timeout, "timeout", "t", 30*time.Second, "Request timeout")
	rootCmd.Flags().DurationVarP(&config.Warmup, "warmup", "w", 5*time.Second, "Warmup duration (0 to disable)")
	rootCmd.Flags().BoolVar(&config.Streaming, "streaming", false, "Test streaming responses")
	rootCmd.Flags().StringVar(&config.PromptSize, "prompt-size", "medium", "Prompt size: small|medium|large|all")
	rootCmd.Flags().StringVarP(&config.Format, "format", "f", "table", "Output format: table|json")
	rootCmd.Flags().BoolVarP(&config.Verbose, "verbose", "v", false, "Verbose output")
	rootCmd.Flags().BoolVar(&config.NoColor, "no-color", false, "Disable colored output")
	rootCmd.Flags().StringVarP(&config.APIKey, "api-key", "k", "", "API key (or use OPENAI_API_KEY env var)")
	rootCmd.Flags().BoolVar(&config.SkipDiscovery, "skip-discovery", false, "Skip model discovery, use specified models directly")
}

func runBenchmark(cmd *cobra.Command, args []string) error {
	config.ServerURL = args[0]
	
	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt, stopping benchmark...")
		cancel()
	}()
	
	// Create and run benchmarker
	benchmarker := NewBenchmarker(config)
	return benchmarker.Run(ctx)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}