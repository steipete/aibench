package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client handles OpenAI API interactions
type Client struct {
	baseURL         string
	httpClient      *http.Client
	availableModels []Model
	apiKey          string
}

// Model represents an OpenAI model
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelsResponse represents the response from /v1/models
type ModelsResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// CompletionRequest represents a chat completion request
type CompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionResponse represents the response from completion API
type CompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
	// Timing information (not from API, added by client)
	RequestTime  time.Time
	ResponseTime time.Time
	TTFT         time.Duration // Time to first token (for streaming)
}

// Choice represents a completion choice
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message,omitempty"`
	Delta        Message `json:"delta,omitempty"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamResponse represents a streaming response chunk
type StreamResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

// isIPAddress checks if a string looks like an IP address (with optional port)
func isIPAddress(s string) bool {
	// Remove port if present
	host, _, err := net.SplitHostPort(s)
	if err != nil {
		host = s // No port, use the whole string
	}
	return net.ParseIP(host) != nil
}

// NewClient creates a new OpenAI API client
func NewClient(serverURL string, timeout time.Duration, apiKey string) *Client {
	// Ensure URL has proper scheme
	if !strings.HasPrefix(serverURL, "http://") && !strings.HasPrefix(serverURL, "https://") {
		// Use HTTP for IP addresses and localhost, HTTPS for domains
		if strings.Contains(serverURL, "localhost") || 
		   strings.HasPrefix(serverURL, "127.") ||
		   isIPAddress(serverURL) {
			serverURL = "http://" + serverURL
		} else {
			serverURL = "https://" + serverURL
		}
	}
	
	// Parse and validate URL
	u, err := url.Parse(serverURL)
	if err != nil {
		// Fallback to basic URL
		if strings.Contains(serverURL, "localhost") || 
		   strings.HasPrefix(serverURL, "127.") ||
		   isIPAddress(serverURL) {
			serverURL = "http://" + serverURL
		} else {
			serverURL = "https://" + serverURL
		}
	} else {
		serverURL = u.String()
	}
	
	// Remove trailing slash
	serverURL = strings.TrimSuffix(serverURL, "/")
	
	// Use environment variable as fallback
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	
	return &Client{
		baseURL: serverURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     30 * time.Second,
			},
		},
	}
}

// ListModels fetches available models from the server
func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	// Handle URLs that already contain versioned paths
	var url string
	if strings.Contains(c.baseURL, "/v1") {
		url = c.baseURL + "/models"
	} else {
		url = c.baseURL + "/v1/models"
	}
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	// Add API key if available
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	
	var modelsResp ModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return modelsResp.Data, nil
}

// CreateCompletion creates a non-streaming completion
func (c *Client) CreateCompletion(ctx context.Context, model, prompt string) (*CompletionResponse, error) {
	reqBody := CompletionRequest{
		Model: model,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
		MaxTokens:   150,
		Temperature: 0.7,
		Stream:      false,
	}
	
	return c.makeCompletionRequest(ctx, reqBody)
}

// CreateStreamingCompletion creates a streaming completion
func (c *Client) CreateStreamingCompletion(ctx context.Context, model, prompt string) (*CompletionResponse, error) {
	reqBody := CompletionRequest{
		Model: model,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
		MaxTokens:   150,
		Temperature: 0.7,
		Stream:      true,
	}
	
	return c.makeStreamingRequest(ctx, reqBody)
}

// makeCompletionRequest handles non-streaming requests
func (c *Client) makeCompletionRequest(ctx context.Context, reqBody CompletionRequest) (*CompletionResponse, error) {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Handle URLs that already contain versioned paths
	var url string
	if strings.Contains(c.baseURL, "/v1") {
		url = c.baseURL + "/chat/completions"
	} else {
		url = c.baseURL + "/v1/chat/completions"
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	// Add API key if available
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	
	requestTime := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	
	responseTime := time.Now()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	
	var completion CompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	completion.RequestTime = requestTime
	completion.ResponseTime = responseTime
	
	return &completion, nil
}

// makeStreamingRequest handles streaming requests
func (c *Client) makeStreamingRequest(ctx context.Context, reqBody CompletionRequest) (*CompletionResponse, error) {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Handle URLs that already contain versioned paths
	var url string
	if strings.Contains(c.baseURL, "/v1") {
		url = c.baseURL + "/chat/completions"
	} else {
		url = c.baseURL + "/v1/chat/completions"
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	// Add API key if available
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	req.Header.Set("Accept", "text/event-stream")
	
	requestTime := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	
	// Process streaming response
	completion, err := c.processStreamingResponse(resp.Body, requestTime)
	if err != nil {
		return nil, err
	}
	
	return completion, nil
}

// processStreamingResponse processes the SSE stream and returns a complete response
func (c *Client) processStreamingResponse(body io.Reader, requestTime time.Time) (*CompletionResponse, error) {
	scanner := bufio.NewScanner(body)
	
	var completion CompletionResponse
	var content strings.Builder
	var firstTokenTime time.Time
	var lastChunk *StreamResponse
	
	completion.RequestTime = requestTime
	
	for scanner.Scan() {
		line := scanner.Text()
		
		// Skip empty lines and non-data lines
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		
		// Extract JSON data
		data := strings.TrimPrefix(line, "data: ")
		
		// Check for end of stream
		if data == "[DONE]" {
			break
		}
		
		var chunk StreamResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // Skip malformed chunks
		}
		
		// Record first token time
		if firstTokenTime.IsZero() && len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			firstTokenTime = time.Now()
			completion.TTFT = firstTokenTime.Sub(requestTime)
		}
		
		// Accumulate content
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			content.WriteString(chunk.Choices[0].Delta.Content)
		}
		
		lastChunk = &chunk
	}
	
	completion.ResponseTime = time.Now()
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading stream: %w", err)
	}
	
	// Build final completion response
	if lastChunk != nil {
		completion.ID = lastChunk.ID
		completion.Object = "chat.completion"
		completion.Created = lastChunk.Created
		completion.Model = lastChunk.Model
		completion.Choices = []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: content.String(),
				},
				FinishReason: "stop",
			},
		}
		
		// Estimate token usage (rough approximation)
		promptTokens := len(strings.Fields(completion.Choices[0].Message.Content)) / 4
		completionTokens := len(strings.Fields(content.String())) / 4
		completion.Usage = Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		}
	}
	
	return &completion, nil
}