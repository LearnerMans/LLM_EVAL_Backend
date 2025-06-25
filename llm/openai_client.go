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

	currentAPIEndpoint := openAIAPIEndpoint // Use the package-level variable

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
	client := &http.Client{Timeout: 60 * time.Second} // This is the client's general timeout

	// Retry logic
	maxRetries := 3
	retryDelay := 2 * time.Second
	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay)
			retryDelay *= 2 // Exponential backoff
		}

		// Prepare request for the current attempt
		// Use the overall context for the request, client.Timeout will apply to individual attempts if shorter
		req, err := http.NewRequestWithContext(ctx, "POST", currentAPIEndpoint, bytes.NewBuffer(jsonBody))
		if err != nil {
			// This error is critical; if NewRequestWithContext fails, it's unlikely to succeed on retry.
			return nil, fmt.Errorf("failed to create HTTP request on attempt %d: %w", attempt+1, err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err = client.Do(req) // client.Timeout applies here for each attempt

		// Check for successful response first
		if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
			break // Success, exit retry loop
		}

		// Handle errors and decide if we should retry
		if err != nil { // Network error or client.Timeout from client.Do(req)
			lastErr = err // Store the error
			fmt.Printf("OpenAI API request failed with client/network error (attempt %d/%d): %v\n", attempt+1, maxRetries, err)
		} else if resp != nil { // HTTP response received, but status code is not OK
			lastErr = fmt.Errorf("OpenAI API returned status: %d", resp.StatusCode) // Store status as error
			fmt.Printf("OpenAI API request failed with status %d (attempt %d/%d)\n", resp.StatusCode, attempt+1, maxRetries)
		}

		// Check if the overall context is done (e.g., the 90s timeout for GenerateContentREST_open)
		if ctx.Err() != nil {
			// If overall context is done, don't retry, return the context error.
			// lastErr might contain more specific info about the last attempt.
			return nil, fmt.Errorf("OpenAI request cancelled or timed out during retry: %w (last attempt error: %v)", ctx.Err(), lastErr)
		}

		shouldRetry := false
		if err != nil { // Analyze network/client errors for retriability
			errMsg := err.Error()
			// Check for client-side timeout errors specifically if client.Timeout is shorter than ctx timeout
			if errMsg == "context deadline exceeded" || // This can be ambiguous (client.Timeout or ctx timeout)
				errMsg == "Client.Timeout exceeded while awaiting headers" || // Specific to client.Timeout
				errMsg == "net/http: timeout awaiting response headers" { // Specific to client.Timeout
				shouldRetry = true // Retry on client-side per-attempt timeouts
			}
			// Add other specific network errors if needed, e.g., temporary DNS issues
		} else if resp != nil { // Analyze HTTP status codes for retriability
			switch resp.StatusCode {
			case http.StatusTooManyRequests, // 429
				http.StatusInternalServerError, // 500
				http.StatusBadGateway,          // 502
				http.StatusServiceUnavailable,  // 503
				http.StatusGatewayTimeout:      // 504
				shouldRetry = true
			}
			// IMPORTANT: Close response body if we are going to retry or if it's a terminal non-OK status
			// to prevent resource leaks.
			if resp.Body != nil {
				resp.Body.Close()
			}
		}

		if !shouldRetry || attempt == maxRetries-1 {
			// If not retrying or if it's the last attempt, break out of the loop.
			// The error handling after the loop will take over.
			break
		}
		// Implicitly continue to next attempt with sleep at the start of the loop
	}

	// After the loop, check the outcome
	if ctx.Err() != nil { // Check overall context timeout first (e.g., 90s budget exceeded)
		// This means the entire operation timed out, possibly spanning multiple retries.
		return nil, fmt.Errorf("OpenAI operation cancelled or timed out after %d attempts. Last error: %v", maxRetries, lastErr)
	}

	if resp == nil { // All attempts failed, likely due to persistent network errors or client.Timeout on all retries
		return nil, fmt.Errorf("request failed after %d attempts. Last error: %w", maxRetries, lastErr)
	}
	// If we are here, resp is not nil. We must ensure its body is closed.
	// If it was closed in the loop (for retriable errors), closing again is harmless.
	// If it was a successful response (200 OK), or a non-retriable error on the last attempt,
	// the body needs to be closed here.
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// This handles cases where the loop exited due to a non-retriable status code,
		// or a retriable one on the final attempt.
		body, readErr := io.ReadAll(resp.Body) // Read body for error message
		if readErr != nil {
			// Failed to read the error body, but we still have the status code.
			return nil, fmt.Errorf("OpenAI API error: status %d, failed to read error body: %w", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("OpenAI API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	// Success: status is http.StatusOK
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
	if raw == "" {
		// Check for empty content as per the main branch logic
		return nil, fmt.Errorf("OpenAI API returned empty message content")
	}

	var output LLMOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		// Use the more informative error message from main
		return nil, fmt.Errorf("failed to unmarshal LLMOutput from OpenAI response: %w. Raw content: %s", err, raw)
	}

	return &output, nil
}
