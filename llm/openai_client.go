package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Package level variable for the OpenAI API endpoint to allow overriding in tests
var openAIAPIEndpoint = "https://api.openai.com/v1/chat/completions"

// ChatCompletionRequest represents the OpenAI chat completion payload.
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// ChatMessage defines a single message for the chat API.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionResponse represents the response from the OpenAI API.
type ChatCompletionResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	// Other fields like usage, created, etc. can be added if needed
}

// GenerateContentREST interacts with the OpenAI Chat API via REST to generate content.
func GenerateContentREST_open(input LLMInput) (*LLMOutput, error) {
	// Set up a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable not set")
	}

	// Use package-level variable for the API endpoint
	// apiEndpoint := "https://api.openai.com/v1/chat/completions" // Original
	currentAPIEndpoint := openAIAPIEndpoint // Use the global variable

	// Marshal input to JSON for user message
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal LLMInput: %w", err)
	}

	// Prepare messages
	systemPrompt := SystemPrompt // define SystemPrompt elsewhere or inline as needed
	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: string(inputJSON)},
	}

	// Build request body
	reqBody := ChatCompletionRequest{
		Model:       "gpt-4",
		Messages:    messages,
		Temperature: 0,
		MaxTokens:   1024,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP client
	client := &http.Client{Timeout: 60 * time.Second}

	// Retry logic
	maxRetries := 3
	retryDelay := 2 * time.Second
	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay)
			retryDelay *= 2
		}

		// Prepare request
		// Prepare request for the current attempt
		req, err := http.NewRequestWithContext(ctx, "POST", currentAPIEndpoint, bytes.NewBuffer(jsonBody))
		if err != nil {
			// This error is critical for this attempt, but loop might retry.
			// However, if NewRequestWithContext fails, it's likely to fail repeatedly.
			// Consider returning error immediately or let retry logic handle it if it's transient (unlikely for this func).
			return nil, fmt.Errorf("failed to create HTTP request on attempt %d: %w", attempt+1, err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err = client.Do(req)
		if err == nil {
			break // Success
		}
		lastErr = err

		// Check if context is done (e.g., timeout) before retrying
		if ctx.Err() != nil {
			return nil, fmt.Errorf("context done during request/retry: %w (last error: %v)", ctx.Err(), lastErr)
		}

		// If it's the last attempt, return the error
		if attempt == maxRetries-1 {
			return nil, fmt.Errorf("request failed after %d attempts: %w", attempt+1, lastErr)
		}
	}

	if resp == nil {
		// This case should ideally be caught by the loop's error handling,
		// but as a safeguard:
		return nil, fmt.Errorf("all request attempts failed, last error: %w", lastErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body) // Read body for error details
		return nil, fmt.Errorf("API error: status %d, body %s", resp.StatusCode, string(body))
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chat response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from API")
	}

	raw := chatResp.Choices[0].Message.Content
	var output LLMOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		return nil, fmt.Errorf("failed to unmarshal LLMOutput: %w", err)
	}

	return &output, nil
}
