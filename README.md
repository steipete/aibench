# aibench - OpenAI API Benchmarking Tool

A Go CLI tool for benchmarking OpenAI-compatible API servers. Automatically discovers available models and measures comprehensive performance metrics including requests per second, tokens per second, and response latency.

## Recent Benchmark Results

### Qwen Coder Models Comparison (30-second tests)

| Provider | Model | Tokens/sec | Success Rate | Avg Latency | Notes |
|----------|-------|------------|--------------|-------------|-------|
| **Cerebras** | qwen-3-coder-480b | **91.93** ⚡ | 26.8% | 996ms | Highest throughput but rate limited |
| **Alibaba Cloud** | qwen3-coder-plus | **38.83** 🛡️ | 88.9% | 3.5s | Best reliability |
| **Custom Server** | qwen3-coder | **33.88** 📈 | 87.5% | 4.2s | Most consistent |

**Winner:** Cerebras for raw speed, Alibaba Cloud for production reliability

### Key Findings
- **Cerebtras API** delivers exceptional performance (91.93 tokens/sec) but has aggressive rate limiting (~73% error rate)
- **Alibaba Cloud** offers the best balance for production use with 38.83 tokens/sec and 88.9% success rate
- **Custom server** provides predictable performance ideal for development/testing

*Benchmarked on 2025-08-01 with identical prompts and conditions*

## Features

- 🚀 **Auto-discovery**: Automatically discovers available models via `/v1/models` endpoint
- 📊 **Comprehensive metrics**: RPS, tokens/sec, latency percentiles, TTFT, error rates
- ⚡ **Concurrent testing**: Auto-detects optimal concurrency or use custom values  
- 🎯 **Multiple prompt sizes**: Test with small, medium, large, or mixed prompts
- 📈 **Real-time progress**: Live progress bar with dynamic tokens/sec and smart status updates
- 🌊 **Streaming support**: Test both regular and streaming completions
- 🎨 **Rich output**: Colored terminal output and JSON export options
- 🔄 **Warmup phase**: Optional warmup to stabilize server performance
- 🔐 **API Key support**: Built-in authentication for cloud providers
- 🌐 **Smart URL handling**: Automatic HTTPS for domains, HTTP for local IPs

## Installation

```bash
# Clone and build
git clone <repo-url>
cd aibench
go build -o aibench

# Or install directly
go install
```

## Usage

### Basic Usage

```bash
# Test a server with default settings (30s duration)
./aibench 86.38.182.74:8000

# Test specific server with custom duration
./aibench --duration 60s localhost:8000

# Test with specific concurrency
./aibench --concurrency 10 --duration 45s api.server.com:8080
```

### Advanced Options

```bash
# Test specific models only
./aibench --models gpt-4,gpt-3.5-turbo server:8000

# Force concurrency 1 (optimal for vLLM/resource-limited servers)
./aibench --concurrency -1 server:8000

# Test with API key authentication
./aibench --api-key "your-key-here" api.provider.com

# Test streaming responses
./aibench --streaming --duration 30s server:8000  

# Test with large prompts and longer timeout
./aibench --prompt-size large --timeout 60s server:8000

# Export results as JSON
./aibench --format json server:8000 > results.json

# Disable colors and warmup
./aibench --no-color --warmup 0 server:8000
```

### Cloud Provider Examples

```bash
# Alibaba DashScope (Qwen models)
./aibench --api-key "sk-your-key" --models qwen3-coder-plus dashscope-intl.aliyuncs.com/compatible-mode

# OpenAI API
./aibench --api-key "sk-your-key" --models gpt-4 api.openai.com

# Any OpenAI-compatible provider
./aibench --api-key "your-key" your-provider.com/v1
```

## Command Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-d, --duration` | `30s` | How long to run the benchmark |
| `-c, --concurrency` | `0` (auto) | Concurrent requests (0=auto, -1=force 1) |
| `-m, --models` | all discovered | Specific models to test |
| `-k, --api-key` | `""` | API key (or use OPENAI_API_KEY env var) |
| `-t, --timeout` | `30s` | Request timeout |
| `-w, --warmup` | `5s` | Warmup duration (0 to disable) |
| `--streaming` | `false` | Test streaming responses |
| `--prompt-size` | `medium` | Prompt size: small\|medium\|large\|all |
| `--skip-discovery` | `false` | Skip model discovery, use specified models |
| `-f, --format` | `table` | Output format: table\|json |
| `-v, --verbose` | `false` | Verbose output |
| `--no-color` | `false` | Disable colored output |

## Real-Time Display

The benchmark shows **live performance metrics** while running:

```
⏳ [████████████████████] 67.3% | RPS: 45.2 | TPS: 1.2k | Reqs: 1356/1400 | ETA: 9s
```

### Display Features:
- **Adaptive status**: Shows different info based on benchmark phase
  - `Waiting for first response...` - When no requests completed yet
  - `Warming up...` - During initial 2 seconds  
  - `RPS: X | TPS: Y` - Live performance metrics
- **Color-coded metrics**: Green (good), yellow (medium), red (low/slow)
- **Recent vs average**: Shows instantaneous speed after warmup
- **Smart progress bar**: Visual completion percentage with ETA

## Output Metrics

### Performance Metrics
- **Requests/sec**: Successful requests per second
- **Tokens/sec**: Total tokens processed per second  
- **Input/Output Tokens/sec**: Breakdown by token type

### Latency Metrics  
- **Average/Min/Max Latency**: Response time statistics
- **P95/P99 Latency**: 95th and 99th percentile latencies
- **TTFT**: Time to first token (streaming only)

### Reliability Metrics
- **Success Rate**: Percentage of successful requests
- **Error Rate**: Percentage of failed requests
- **Error Breakdown**: Detailed error categorization

## Example Output

```
🚀 aibench - OpenAI API Benchmarking Tool
Server: localhost:8000
Models: gpt-4, gpt-3.5-turbo
────────────────────────────────────────────────────────────

📊 Benchmarking model: gpt-4
ℹ Finding optimal concurrency...
ℹ Optimal concurrency: 8
ℹ Warming up (5s)...
ℹ Running benchmark (concurrency: 8, duration: 30s)...

⏳ [████████████████████] 100.0% | RPS: 45.2 | TPS: 1.2k | Reqs: 1356/1356 | ETA: 0s

📈 Benchmark Results

Model: gpt-4
──────────────────────────────────────────────────
Duration:            30.0s
Total Requests:      1356
Successful:          1356 (100.0% success rate)

Requests/sec:        45.2
Tokens/sec:          1.2k
Input Tokens/sec:    890.5
Output Tokens/sec:   312.1

Avg Latency:         177ms
Min Latency:         89ms  
Max Latency:         945ms
P95 Latency:         298ms
P99 Latency:         567ms
Avg TTFT:            45ms
```

## API Compatibility

This tool works with any OpenAI-compatible API server including:
- OpenAI API
- Local LLM servers (Ollama, LM Studio, etc.)
- Cloud providers (Azure OpenAI, AWS Bedrock, Alibaba DashScope, etc.)
- Custom inference servers

### Required Endpoints:
- `POST /v1/chat/completions` - Chat completions (required)
- `GET /v1/models` - Model discovery (optional)

### Model Discovery Issues

**Important**: Some providers have incomplete or missing model discovery endpoints. Common issues:

1. **Missing Models**: The `/v1/models` endpoint may not list all available models
2. **Authentication Issues**: Discovery may fail even when the completion endpoint works
3. **Provider Limitations**: Some APIs don't implement the models endpoint

**Solutions**:
- **Specify models directly**: Use `--models model-name` to bypass discovery
- **Skip discovery**: Use `--skip-discovery` when you know the model exists
- **Check provider docs**: Consult your API provider's documentation for correct model names

### Automatic Discovery Skipping

**Smart Behavior**: When you specify models with `--models`, discovery is automatically skipped. This eliminates the need for a separate flag and handles provider limitations seamlessly.

```bash
# ❌ This may fail if discovery is broken
./aibench --api-key "key" provider.com

# ✅ This automatically skips discovery (recommended)
./aibench --api-key "key" --models qwen3-coder-plus provider.com

# ✅ Multiple models also skip discovery
./aibench --api-key "key" --models model1,model2,model3 provider.com

# ✅ Only uses discovery when no models specified
./aibench --api-key "key" provider.com  # Will discover all available models
```

**Best Practice**: Always specify model names when working with cloud providers to avoid discovery issues and get faster startup times.

## Architecture

- **Client**: HTTP client with connection pooling for API interactions
- **Benchmarker**: Coordinates the entire benchmark process
- **Metrics**: Thread-safe performance tracking and statistical calculations  
- **Display**: Real-time progress and formatted result output
- **Workers**: Concurrent request generators with different prompt strategies

## License

MIT License