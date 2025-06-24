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

// LLMInput defines the structure for the input JSON to the LLM.
type LLMInput struct {
	Scenario        string       `json:"scenario"`
	ExpectedOutcome string       `json:"expected_outcome"`
	CurrentState    CurrentState `json:"current_state"`
	Version         string       `json:"version"`
}

// CurrentState defines the current state within the LLMInput.
type CurrentState struct {
	History   []HistoryItem `json:"history"`
	TurnCount int16         `json:"turn_count"`
	MaxTurns  int16         `json:"max_turns"`
	Fulfilled bool          `json:"fulfilled"`
}

// HistoryItem defines an item in the conversation history.
type HistoryItem struct {
	Turn      int16  `json:"turn"`
	User      string `json:"user"`
	Assistant string `json:"assistant"`
}

// LLMOutput defines the structure for the LLM's response, based on the specified schema.
type LLMOutput struct {
	NextMessage     string   `json:"next_message"`
	Reasoning       string   `json:"reasoning"`
	Fulfilled       bool     `json:"fulfilled"`
	Confidence      string   `json:"confidence"`
	Strategy        string   `json:"strategy"`
	SafetyCheck     string   `json:"safety_check"`
	ErrorLogs       []string `json:"error_logs"`
	AdaptationNotes string   `json:"adaptation_notes"`
}

// GeminiAPIRequest represents the structure of the request body for the Gemini generateContent API.
type GeminiAPIRequest struct {
	Contents          []Content         `json:"contents"`
	SystemInstruction *Content          `json:"system_instruction,omitempty"`
	GenerationConfig  *GenerationConfig `json:"generation_config,omitempty"`
}

// Content represents a single piece of content (e.g., user input, model response).
type Content struct {
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
}

// Part represents a part of content, e.g., text, inline_data.
type Part struct {
	Text string `json:"text,omitempty"`
	// For image, video, etc., you would add other fields like InlineData
}

// GenerationConfig represents the generation configuration for the model.
type GenerationConfig struct {
	Temperature      float64 `json:"temperature,omitempty"`
	ResponseMIMEType string  `json:"response_mime_type,omitempty"`
	ResponseSchema   *Schema `json:"response_schema,omitempty"`
	// Other fields like TopK, TopP, MaxOutputTokens can be added here
}

// Schema defines the expected structure of the JSON output.
type Schema struct {
	Type       string             `json:"type"`
	Properties map[string]*Schema `json:"properties,omitempty"`
	Items      *Schema            `json:"items,omitempty"` // For array types
}

// GeminiAPIResponse represents the structure of the response from the Gemini generateContent API.
type GeminiAPIResponse struct {
	Candidates []struct {
		Content Content `json:"content"`
		// Other fields like FinishReason, SafetyRatings
	} `json:"candidates"`
	// Other fields like PromptFeedback
}

// GenerateContentREST interacts with the Gemini LLM via REST API to generate content based on the input.
// It takes an LLMInput struct and returns an LLMOutput struct or an error.
func GenerateContentREST(input LLMInput) (*LLMOutput, error) {
	// Set up a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}

	modelName := "gemini-2.5-flash"
	apiEndpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", modelName, apiKey)

	// Convert the LLMInput struct to a JSON string for the user prompt
	inputJSONBytes, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input to JSON: %w", err)
	}
	userPrompt := string(inputJSONBytes)

	// Define the system instruction
	systemInstructionText := SystemPrompt

	// Construct the request body
	requestBody := GeminiAPIRequest{
		Contents: []Content{
			{
				Role: "user",
				Parts: []Part{
					{Text: userPrompt},
				},
			},
		},
		SystemInstruction: &Content{ // System instruction is a Content object with "system" role
			Role: "system",
			Parts: []Part{
				{Text: systemInstructionText},
			},
		},
		GenerationConfig: &GenerationConfig{
			Temperature:      0,
			ResponseMIMEType: "application/json",
			ResponseSchema: &Schema{
				Type: "OBJECT",
				Properties: map[string]*Schema{
					"next_message": {Type: "STRING"},
					"reasoning":    {Type: "STRING"},
					"fulfilled":    {Type: "BOOLEAN"},
					"confidence":   {Type: "STRING"},
					"strategy":     {Type: "STRING"},
					"safety_check": {Type: "STRING"},
					"error_logs": {
						Type:  "ARRAY",
						Items: &Schema{Type: "STRING"},
					},
					"adaptation_notes": {Type: "STRING"},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal API request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiEndpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// The API key is already in the URL for simplicity as per common REST examples for GenAI,
	// but for production, typically passed as a custom header or via auth mechanisms.
	// For this specific API, passing in URL parameter `key` is standard.

	// Set a longer timeout for the request to handle potential network delays
	client := &http.Client{Timeout: 60 * time.Second}

	// Implement retry logic for transient errors
	maxRetries := 3
	retryDelay := 2 * time.Second

	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Check if context is done before making the request
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("operation canceled or timed out: %w", ctx.Err())
		default:
			// Continue with the request
		}

		// If this is a retry, wait before trying again
		if attempt > 0 {
			fmt.Printf("Retrying Gemini API request (attempt %d/%d) after error: %v\n",
				attempt+1, maxRetries, lastErr)
			time.Sleep(retryDelay)
			// Exponential backoff
			retryDelay = retryDelay * 2
		}

		// Create a fresh request for each attempt
		reqCopy, err := http.NewRequestWithContext(ctx, "POST", apiEndpoint, bytes.NewBuffer(jsonBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request: %w", err)
		}
		reqCopy.Header.Set("Content-Type", "application/json")

		resp, err = client.Do(reqCopy)
		if err == nil {
			// Success! Break out of the retry loop
			break
		}

		// Store the error for potential reporting
		lastErr = err

		// Check if this is a timeout or a retriable HTTP status code
		shouldRetry := false
		if err != nil { // Network error or client timeout
			errMsg := err.Error()
			if errMsg == "context deadline exceeded" ||
				errMsg == "Client.Timeout exceeded while awaiting headers" ||
				errMsg == "net/http: timeout awaiting response headers" {
				shouldRetry = true
				fmt.Printf("Gemini API request timed out: %v\n", err)
			} else {
				// For other errors, don't retry immediately, could be a persistent issue
				fmt.Printf("Gemini API request failed with network/client error: %v\n", err)
			}
		} else if resp != nil { // HTTP response received, check status code
			switch resp.StatusCode {
			case http.StatusTooManyRequests, // 429
				http.StatusInternalServerError, // 500
				http.StatusBadGateway,          // 502
				http.StatusServiceUnavailable,  // 503
				http.StatusGatewayTimeout:      // 504
				shouldRetry = true
				// It's good practice to close the response body of a failed request
				// if we are going to retry, to free up resources.
				if resp.Body != nil {
					resp.Body.Close()
				}
				fmt.Printf("Gemini API request failed with status %d, will retry.\n", resp.StatusCode)
			default:
				// Non-retriable HTTP status or success (which would have broken the loop)
				// If it's a non-OK status that we don't retry, we'll handle it after the loop.
			}
		}

		// If we shouldn't retry or we've used all our retries, break the loop.
		// The error/response will be handled outside the loop.
		if !shouldRetry || attempt == maxRetries-1 {
			break
		}
		// Otherwise, we'll retry (after sleep)
	}

	// After the loop, check the outcome
	if lastErr != nil && resp == nil { // Indicates network/client errors for all attempts
		// Check if the overall context timed out
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("operation canceled or timed out after %d attempts: %w. Last error: %v", maxRetries, ctx.Err(), lastErr)
		default:
			return nil, fmt.Errorf("all %d request attempts failed due to network/client errors, last error: %w", maxRetries, lastErr)
		}
	}
	if resp == nil { // Should not happen if lastErr is nil, but as a safeguard
		return nil, fmt.Errorf("no response received from Gemini API after %d attempts", maxRetries)
	}
	defer resp.Body.Close() // Ensure body is closed here for the successful or final failed response

	if resp.StatusCode != http.StatusOK {
		responseBodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("API returned non-OK status: %d, and failed to read error body: %w", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("API returned non-OK status: %d, body: %s", resp.StatusCode, string(responseBodyBytes))
	}

	responseBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var geminiResponse GeminiAPIResponse
	err = json.Unmarshal(responseBodyBytes, &geminiResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal Gemini API response: %w", err)
	}

	if len(geminiResponse.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates returned from LLM API")
	}
	if len(geminiResponse.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no content parts in the first candidate from LLM API")
	}

	rawJSONOutput := geminiResponse.Candidates[0].Content.Parts[0].Text
	// It's possible for the LLM to return an empty JSON string, e.g. "{}" if instructed or if something goes wrong.
	// Or it could be an empty text string "" if the model is misbehaving.
	// Unmarshaling "" into a struct will not error but result in zero values.
	// Unmarshaling "{}" is valid.
	// We should decide if an empty text string for rawJSONOutput is an error.
	// For now, let's assume an empty text string from the Part is an issue.
	if rawJSONOutput == "" {
		// Consider if this should be a more structured error or a specific LLMOutput state
		return nil, fmt.Errorf("LLM API returned empty text content in the response part")
	}

	var output LLMOutput
	err = json.Unmarshal([]byte(rawJSONOutput), &output)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal final LLM output schema: %w", err)
	}

	return &output, nil
}
