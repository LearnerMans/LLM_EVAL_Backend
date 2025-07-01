package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CohereChatRequest represents the request body for Cohere chat API
// Only the fields we need for this use case
// See: https://docs.cohere.com/reference/chat

type CohereChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CohereResponseFormat struct {
	Type       string                 `json:"type"`
	JSONSchema map[string]interface{} `json:"jsonSchema"`
}

type CohereChatRequest struct {
	Messages       []CohereChatMessage  `json:"messages"`
	Temperature    float64              `json:"temperature"`
	Model          string               `json:"model"`
	ResponseFormat CohereResponseFormat `json:"response_format"`
}

// CohereChatResponse represents the response from Cohere chat API
// Only the fields we need

type CohereChatResponse struct {
	ID           string `json:"id"`
	FinishReason string `json:"finish_reason"`
	Message      struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
	Usage interface{} `json:"usage"` // You can define this more strictly if you need usage info
}

// GenerateContentREST implements LLM for CohereClient
func (c *CohereClient) GenerateContentREST(prompt string, input LLMInput) (*LLMOutput, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ContextTimeout)
	defer cancel()

	apiKey := c.apiKey
	if apiKey == "" {
		return nil, fmt.Errorf("COHERE_API_KEY environment variable not set")
	}

	apiEndpoint := "https://api.cohere.com/v2/chat"

	// Prepare the system message
	systemPrompt := prompt

	// Prepare the user message (input as JSON string)
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal LLMInput: %w", err)
	}

	messages := []CohereChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: string(inputJSON)},
	}

	// Prepare the JSON schema for the response_format
	jsonSchema := map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"properties": map[string]interface{}{
			"next_message": map[string]interface{}{
				"type":        "string",
				"description": "Your next message to send to the Knovvu VA",
			},
			"reasoning": map[string]interface{}{
				"type":        "string",
				"description": "Brief explanation of your strategy for this turn",
			},
			"fulfilled": map[string]interface{}{
				"type":        "boolean",
				"description": "Indicates whether the objective of this turn has been fulfilled",
			},
			"confidence": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"high", "medium", "low"},
				"description": "Level of confidence in the response or strategy",
			},
			"strategy": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"direct", "exploratory", "clarification", "escalation", "alternative"},
				"description": "The approach or tactic being used for this interaction",
			},
			"safety_check": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"passed", "flagged"},
				"description": "Result of the safety check for this interaction",
			},
			"error_logs": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "List of any unexpected behaviors or responses to log",
			},
			"adaptation_notes": map[string]interface{}{
				"type":        "string",
				"description": "Notes on how you're adapting based on observed VA patterns",
			},
		},
		"required": []string{"next_message", "reasoning", "fulfilled", "confidence", "strategy", "safety_check", "error_logs", "adaptation_notes"},
	}

	requestBody := CohereChatRequest{
		Messages:       messages,
		Temperature:    0,
		Model:          c.Model,
		ResponseFormat: CohereResponseFormat{Type: "json_object", JSONSchema: jsonSchema},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}

	var resp *http.Response
	var lastErr error
	maxRetries := 3
	retryDelay := 2 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay)
			retryDelay *= 2
		}

		req, err := http.NewRequestWithContext(ctx, "POST", apiEndpoint, bytes.NewBuffer(jsonBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err = client.Do(req)
		if err == nil {
			break
		}
		lastErr = err
		if ctx.Err() != nil || attempt == maxRetries-1 {
			return nil, fmt.Errorf("request failed after %d attempts: %w", attempt+1, err)
		}
	}

	if resp == nil {
		return nil, fmt.Errorf("all attempts failed, last error: %w", lastErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: status %d, body %s", resp.StatusCode, string(body))
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// The Cohere API returns the JSON output as a string in the 'text' field
	var chatResp CohereChatResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chat response: %w", err)
	}

	if chatResp.Message.Content[0].Text == "" {
		return nil, fmt.Errorf("empty text field in Cohere API response")
	}

	var output LLMOutput
	if err := json.Unmarshal([]byte(chatResp.Message.Content[0].Text), &output); err != nil {
		return nil, fmt.Errorf("failed to unmarshal LLMOutput: %w", err)
	}

	return &output, nil
}

func (c *CohereClient) GenerateJudgmentREST(judgePrompt string, input JudgeInput) (*JudgmentResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ContextTimeout)
	defer cancel()

	apiKey := c.apiKey
	if apiKey == "" {
		return nil, fmt.Errorf("COHERE_API_KEY environment variable not set")
	}

	apiEndpoint := "https://api.cohere.com/v2/chat"

	// System prompt as in your agent design (give Judge instructions here)
	systemPrompt := judgePrompt

	// User message is the JudgeInput as JSON
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JudgeInput: %w", err)
	}

	messages := []CohereChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: string(inputJSON)},
	}

	// JSON schema for JudgmentResult
	jsonSchema := map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"properties": map[string]interface{}{
			"judgment": map[string]interface{}{
				"type":        "string",
				"description": "Final verdict on whether the scenario was completed as intended",
			},
			"confidence": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"high", "medium", "low"},
				"description": "Level of confidence in the judgment",
			},
			"evidence_summary": map[string]interface{}{
				"type":        "string",
				"description": "Concise summary of evidence from the conversation that led to the verdict",
			},
			"scenario_completion_score": map[string]interface{}{
				"type":        "number",
				"description": "Score 0-1 for scenario completion",
			},
			"conversation_quality_score": map[string]interface{}{
				"type":        "number",
				"description": "Score 0-1 for overall conversation quality",
			},
		},
		"required": []string{"judgment", "confidence", "evidence_summary", "scenario_completion_score", "conversation_quality_score"},
	}

	requestBody := CohereChatRequest{
		Messages:       messages,
		Temperature:    0,
		Model:          c.Model,
		ResponseFormat: CohereResponseFormat{Type: "json_object", JSONSchema: jsonSchema},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}

	var resp *http.Response
	var lastErr error
	maxRetries := 3
	retryDelay := 2 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay)
			retryDelay *= 2
		}

		req, err := http.NewRequestWithContext(ctx, "POST", apiEndpoint, bytes.NewBuffer(jsonBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err = client.Do(req)
		if err == nil {
			break
		}
		lastErr = err
		if ctx.Err() != nil || attempt == maxRetries-1 {
			return nil, fmt.Errorf("request failed after %d attempts: %w", attempt+1, err)
		}
	}

	if resp == nil {
		return nil, fmt.Errorf("all attempts failed, last error: %w", lastErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: status %d, body %s", resp.StatusCode, string(body))
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var chatResp CohereChatResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chat response: %w", err)
	}

	if len(chatResp.Message.Content) == 0 || chatResp.Message.Content[0].Text == "" {
		return nil, fmt.Errorf("empty text field in Cohere API response")
	}

	var result JudgmentResult
	if err := json.Unmarshal([]byte(chatResp.Message.Content[0].Text), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JudgmentResult: %w", err)
	}

	return &result, nil
}
