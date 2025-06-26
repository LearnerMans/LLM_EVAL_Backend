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
func (c *GeminiClient) GenerateContentREST(input LLMInput) (*LLMOutput, error) {
	// Set up a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), ContextTimeout)
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

		// Check if this is a timeout or transient error that's worth retrying
		errMsg := err.Error()
		isTimeout := errMsg == "context deadline exceeded" ||
			errMsg == "Client.Timeout exceeded while awaiting headers" ||
			errMsg == "net/http: timeout awaiting response headers"

		// If it's not a timeout or we've used all our retries, return the error
		if !isTimeout || attempt == maxRetries-1 {
			if isTimeout {
				return nil, fmt.Errorf("HTTP request timed out after %d attempts. This could be due to network issues or the Gemini API being overloaded. Please try again later", maxRetries)
			}
			return nil, fmt.Errorf("HTTP request failed after %d attempts: %w", attempt+1, err)
		}
		// Otherwise, we'll retry
	}

	// If all retries failed
	if resp == nil {
		return nil, fmt.Errorf("all %d request attempts failed, last error: %w", maxRetries, lastErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBodyBytes, _ := io.ReadAll(resp.Body)
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
	if rawJSONOutput == "" {
		return nil, fmt.Errorf("empty text content in the first candidate from LLM API")
	}

	var output LLMOutput
	err = json.Unmarshal([]byte(rawJSONOutput), &output)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal final LLM output schema: %w", err)
	}

	return &output, nil
}
