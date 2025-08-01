package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pterm/pterm"
)

// Display handles all output formatting and progress display
type Display struct {
	noColor bool
}

// NewDisplay creates a new display handler
func NewDisplay(noColor bool) *Display {
	d := &Display{noColor: noColor}
	
	if noColor {
		pterm.DisableColor()
	}
	
	return d
}

// PrintHeader displays the benchmark header
func (d *Display) PrintHeader(serverURL string, models []string) {
	fmt.Println()
	pterm.DefaultHeader.WithFullWidth().WithBackgroundStyle(pterm.NewStyle(pterm.BgBlue)).WithTextStyle(pterm.NewStyle(pterm.FgWhite)).Println("🚀 aibench - OpenAI API Benchmarking Tool")
	pterm.Printf("%s %s\n", pterm.LightCyan("Server:"), serverURL)
	pterm.Printf("%s %s\n", pterm.LightCyan("Models:"), strings.Join(models, ", "))
	fmt.Printf("%s\n\n", strings.Repeat("─", 60))
}

// PrintModelHeader displays the header for a specific model benchmark
func (d *Display) PrintModelHeader(model string) {
	fmt.Println()
	pterm.Info.Printf("Benchmarking model: %s\n", pterm.LightBlue(model))
}

// PrintStatus displays a status message
func (d *Display) PrintStatus(message string) {
	pterm.Info.Println(message)
}

// PrintError displays an error message
func (d *Display) PrintError(message string) {
	pterm.Error.Println(message)
}

// ShowProgress displays real-time progress during benchmarking using pterm
func (d *Display) ShowProgress(ctx context.Context, metrics *Metrics, duration time.Duration) {
	startTime := time.Now()  
	liveMetrics := NewLiveMetrics(metrics)
	
	// Create pterm progress bar
	p, _ := pterm.DefaultProgressbar.WithTotal(int(duration.Seconds())).WithTitle("Running benchmark").Start()
	
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	progressDone := make(chan struct{})
	
	go func() {
		defer close(progressDone)
		for {
			select {
			case <-ticker.C:
				elapsed := time.Since(startTime)
				remaining := duration - elapsed
				
				if remaining <= 0 {
					if p.Current < p.Total {
						p.Add(p.Total - p.Current) // Set to 100%
					}
					continue // Keep updating the title even after 100%
				}
				
				// Get current stats
				_, tps, successful, total, _ := liveMetrics.GetLiveStats()
				
				// Simple title showing requests and tokens/sec
				var title string
				if successful > 0 && tps > 0 {
					title = fmt.Sprintf("Running benchmark... (Reqs: %d | %.2f Tokens/sec)", total, tps)
				} else {
					title = fmt.Sprintf("Running benchmark... (Reqs: %d)", total)
				}
				
				p.UpdateTitle(title)
				if p.Current < p.Total {
					p.Add(1)
				}
				
			case <-ctx.Done():
				// Set to 100% when context is done
				if p.Current < p.Total {
					p.Add(p.Total - p.Current)
				}
				return
			}
		}
	}()
	
	// Wait for context to be done
	<-ctx.Done()
	
	// Wait for the progress goroutine to finish
	<-progressDone
	
	// Keep the completed progress bar visible for a moment
	time.Sleep(1 * time.Second)
	
	// Now stop the progress bar
	p.Stop()
}

// PrintResults displays the final benchmark results
func (d *Display) PrintResults(results []BenchmarkResult, format string) {
	fmt.Print("\n\n")
	
	switch format {
	case "json":
		d.printJSONResults(results)
	default:
		d.printTableResults(results)
	}
}

// printTableResults displays results in a formatted table
func (d *Display) printTableResults(results []BenchmarkResult) {
	fmt.Println()
	pterm.DefaultSection.Println("📈 Benchmark Results")
	
	if len(results) == 0 {
		pterm.Warning.Println("No benchmark results to display.")
		return
	}
	
	// Create table data
	tableData := pterm.TableData{
		{"Model", "Tokens/sec", "Reqs/sec", "Success Rate", "Avg Latency", "P95 Latency"},
	}
	
	for _, result := range results {
		successRate := 100 - result.ErrorRate
		tableData = append(tableData, []string{
			result.Model,
			fmt.Sprintf("%.2f", result.TokensPerSec),
			fmt.Sprintf("%.2f", result.RequestsPerSec),
			fmt.Sprintf("%.1f%%", successRate),
			d.formatDuration(result.AvgLatency),
			d.formatDuration(result.P95Latency),
		})
	}
	
	pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
	
	// Show detailed results for each model
	for _, result := range results {
		d.printModelResult(result)
		fmt.Println()
	}
}

// printModelResult displays results for a single model
func (d *Display) printModelResult(result BenchmarkResult) {
	pterm.Printf("\n%s %s\n", pterm.Bold.Sprint("Model:"), pterm.LightBlue(result.Model))
	fmt.Printf("%s\n", strings.Repeat("─", 50))
	
	// Performance metrics
	pterm.Printf("%-20s %s\n", "Duration:", d.formatDuration(result.Duration))
	pterm.Printf("%-20s %s\n", "Total Requests:", d.formatInt(result.TotalRequests))
	pterm.Printf("%-20s %s (%s success rate)\n", 
		"Successful:", 
		d.formatInt(result.SuccessfulReqs),
		d.formatPercentage(100-result.ErrorRate))
	
	if result.FailedReqs > 0 {
		pterm.Printf("%-20s %s (%s)\n", 
			"Failed:", 
			pterm.Red(d.formatInt(result.FailedReqs)),
			pterm.Red(d.formatPercentage(result.ErrorRate)))
	}
	
	fmt.Println()
	
	// Throughput metrics
	pterm.Printf("%-20s %s\n", "Requests/sec:", pterm.Green(d.formatNumber(result.RequestsPerSec)))
	pterm.Printf("%-20s %s\n", "Tokens/sec:", pterm.Green(d.formatNumber(result.TokensPerSec)))
	pterm.Printf("%-20s %s\n", "Input Tokens/sec:", d.formatNumber(result.InputTokensPerSec))
	pterm.Printf("%-20s %s\n", "Output Tokens/sec:", d.formatNumber(result.OutputTokensPerSec))
	
	fmt.Println()
	
	// Latency metrics
	pterm.Printf("%-20s %s\n", "Avg Latency:", d.formatDuration(result.AvgLatency))
	pterm.Printf("%-20s %s\n", "Min Latency:", d.formatDuration(result.MinLatency))
	pterm.Printf("%-20s %s\n", "Max Latency:", d.formatDuration(result.MaxLatency))
	pterm.Printf("%-20s %s\n", "P95 Latency:", d.formatDuration(result.P95Latency))
	pterm.Printf("%-20s %s\n", "P99 Latency:", d.formatDuration(result.P99Latency))
	
	if result.AvgTTFT > 0 {
		pterm.Printf("%-20s %s\n", "Avg TTFT:", d.formatDuration(result.AvgTTFT))
	}
	
	// Error breakdown
	if len(result.Errors) > 0 {
		fmt.Println()
		pterm.Printf("%s\n", pterm.Bold.Sprint("Errors:"))
		for errMsg, count := range result.Errors {
			pterm.Printf("  %s %s: %d\n", pterm.Red("•"), errMsg, count)
		}
	}
}

// printJSONResults outputs results in JSON format
func (d *Display) printJSONResults(results []BenchmarkResult) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.Encode(results)
}

// Formatting helper functions

func (d *Display) formatNumber(value float64) string {
	if value >= 1000 {
		return fmt.Sprintf("%.1fk", value/1000)
	}
	return fmt.Sprintf("%.1f", value)
}

func (d *Display) formatLiveNumber(value float64) string {
	if value >= 1000 {
		return pterm.Green(fmt.Sprintf("%.1fk", value/1000))
	} else if value >= 10 {
		return pterm.Green(fmt.Sprintf("%.1f", value))
	} else if value >= 1 {
		return pterm.Yellow(fmt.Sprintf("%.2f", value))
	} else if value > 0 {
		return pterm.Red(fmt.Sprintf("%.3f", value))
	}
	return pterm.Red("0.0")
}

func (d *Display) formatInt(value int64) string {
	if value >= 1000 {
		return fmt.Sprintf("%.1fk", float64(value)/1000)
	}
	return fmt.Sprintf("%d", value)
}

func (d *Display) formatDuration(duration time.Duration) string {
	if duration >= time.Second {
		return fmt.Sprintf("%.1fs", duration.Seconds())
	} else if duration >= time.Millisecond {
		return fmt.Sprintf("%.0fms", duration.Seconds()*1000)
	} else {
		return fmt.Sprintf("%.0fμs", duration.Seconds()*1000000)
	}
}

func (d *Display) formatPercentage(value float64) string {
	return fmt.Sprintf("%.1f%%", value)
}

// PrintSummary displays a quick summary at the end
func (d *Display) PrintSummary(results []BenchmarkResult) {
	if len(results) == 0 {
		return
	}
	
	pterm.DefaultSection.Println("📋 Summary")
	
	var totalRPS, totalTPS float64
	bestModel := ""
	bestRPS := 0.0
	
	for _, result := range results {
		totalRPS += result.RequestsPerSec
		totalTPS += result.TokensPerSec
		
		if result.RequestsPerSec > bestRPS {
			bestRPS = result.RequestsPerSec
			bestModel = result.Model
		}
	}
	
	pterm.Printf("Models tested: %s\n", pterm.Bold.Sprintf("%d", len(results)))
	pterm.Printf("Total RPS: %s\n", pterm.Green(d.formatNumber(totalRPS)))
	pterm.Printf("Total TPS: %s\n", pterm.Green(d.formatNumber(totalTPS)))
	
	if bestModel != "" {
		pterm.Printf("Best performing: %s (%s RPS)\n", 
			pterm.LightBlue(bestModel), 
			pterm.Green(d.formatNumber(bestRPS)))
	}
	
	fmt.Println()
}