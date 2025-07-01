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
func (c *OpenAIClient) GenerateContentREST(prompt string, input LLMInput) (*LLMOutput, error) {
	// Set up a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), ContextTimeout)
	defer cancel()

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable not set")
	}

	apiEndpoint := "https://api.openai.com/v1/chat/completions"

	// Marshal input to JSON for user message
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal LLMInput: %w", err)
	}

	// Prepare messages
	systemPrompt := prompt // define SystemPrompt elsewhere or inline as needed
	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: string(inputJSON)},
	}

	// Build request body
	reqBody := ChatCompletionRequest{
		Model:       c.Model,
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
		req, err := http.NewRequestWithContext(ctx, "POST", apiEndpoint, bytes.NewBuffer(jsonBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err = client.Do(req)
		if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
			// Success
			break
		}
		// If err is not nil, or status code is not OK.
		if err != nil {
			lastErr = err // Store network/client error
			fmt.Printf("OpenAI API request failed with client/network error (attempt %d/%d): %v\n", attempt+1, maxRetries, err)
		} else if resp != nil { // err is nil, but status code is not OK
			// Store the response status code as an error to potentially retry
			lastErr = fmt.Errorf("OpenAI API returned status: %d", resp.StatusCode)
			fmt.Printf("OpenAI API request failed with status %d (attempt %d/%d)\n", resp.StatusCode, attempt+1, maxRetries)
		}

		shouldRetry := false
		if ctx.Err() != nil { // Overall context cancelled
			return nil, fmt.Errorf("OpenAI request cancelled or timed out during retry: %w", ctx.Err())
		}

		if err != nil { // Network error or client timeout from client.Do(req)
			errMsg := err.Error()
			if errMsg == "context deadline exceeded" || // This refers to client.Timeout
				errMsg == "Client.Timeout exceeded while awaiting headers" ||
				errMsg == "net/http: timeout awaiting response headers" {
				shouldRetry = true
			}
		} else if resp != nil { // HTTP response received, check status code for retry
			switch resp.StatusCode {
			case http.StatusTooManyRequests, // 429
				http.StatusInternalServerError, // 500
				http.StatusBadGateway,          // 502
				http.StatusServiceUnavailable,  // 503
				http.StatusGatewayTimeout:      // 504
				shouldRetry = true
			}
			// Close non-nil response body if we are retrying or if it's a terminal non-OK status
			if resp.Body != nil {
				resp.Body.Close()
			}
		}

		if !shouldRetry || attempt == maxRetries-1 {
			// If not retrying or last attempt, break to handle error outside loop
			break
		}
		// Sleep before next attempt only if retrying and not the last attempt
		// (Handled by the time.Sleep at the start of the loop for attempt > 0)
	}

	// After the loop, check the outcome
	if ctx.Err() != nil { // Check overall context timeout first
		return nil, fmt.Errorf("OpenAI operation cancelled or timed out after %d attempts. Last error: %v", maxRetries, lastErr)
	}

	if resp == nil { // All attempts failed, possibly due to network errors
		return nil, fmt.Errorf("all %d OpenAI request attempts failed. Last error: %w", maxRetries, lastErr)
	}
	// If we are here, resp is not nil. We must close its body.
	// If it was closed in the loop (for retriable errors), closing again is fine.
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body) // Read body for error message
		if readErr != nil {
			return nil, fmt.Errorf("OpenAI API error: status %d, failed to read error body: %w", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("OpenAI API error: status %d, body: %s", resp.StatusCode, string(body))
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
	if raw == "" {
		// Similar to Gemini client, if the content string is empty,
		// unmarshalling it would lead to zero-values, which might be misleading.
		return nil, fmt.Errorf("OpenAI API returned empty message content")
	}

	var output LLMOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		return nil, fmt.Errorf("failed to unmarshal LLMOutput from OpenAI response: %w. Raw content: %s", err, raw)
	}

	return &output, nil
}
