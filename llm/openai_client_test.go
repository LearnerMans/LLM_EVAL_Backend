package llm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	// "time" // No longer used directly in this test file
)

// TestGenerateContentREST_OpenAI_Success tests successful API call for OpenAI
func TestGenerateContentREST_OpenAI_Success(t *testing.T) {
	expectedOutput := &LLMOutput{
		NextMessage: "Hello from OpenAI",
		Reasoning:   "Test reasoning from OpenAI",
		Fulfilled:   false,
		Confidence:  "medium",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check auth header
		if r.Header.Get("Authorization") != "Bearer test_openai_key" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check request body
		var reqBody ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			http.Error(w, "Cannot decode body", http.StatusBadRequest)
			return
		}
		if reqBody.Model != "gpt-4" {
			http.Error(w, "Unexpected model", http.StatusBadRequest)
			return
		}
		if len(reqBody.Messages) != 2 || reqBody.Messages[0].Role != "system" || reqBody.Messages[1].Role != "user" {
			http.Error(w, "Unexpected messages structure", http.StatusBadRequest)
			return
		}

		// Prepare response
		apiResp := ChatCompletionResponse{
			Choices: []struct {
				Message ChatMessage `json:"message"`
			}{
				{
					Message: ChatMessage{
						Role:    "assistant",
						Content: marshalJSONToStringForOpenAI(expectedOutput),
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(apiResp)
	}))
	defer server.Close()

	originalEndpoint := openAIAPIEndpoint
	openAIAPIEndpoint = server.URL
	defer func() { openAIAPIEndpoint = originalEndpoint }()

	os.Setenv("OPENAI_API_KEY", "test_openai_key")
	defer os.Unsetenv("OPENAI_API_KEY")

	input := LLMInput{Scenario: "OpenAI Test"}
	output, err := GenerateContentREST_open(input)

	if err != nil {
		t.Fatalf("GenerateContentREST_open returned error: %v", err)
	}
	if output == nil {
		t.Fatalf("GenerateContentREST_open returned nil output")
	}
	if output.NextMessage != expectedOutput.NextMessage {
		t.Errorf("Expected NextMessage %q, got %q", expectedOutput.NextMessage, output.NextMessage)
	}
	if output.Reasoning != expectedOutput.Reasoning {
		t.Errorf("Expected Reasoning %q, got %q", expectedOutput.Reasoning, output.Reasoning)
	}
}

// TestGenerateContentREST_OpenAI_APIError tests API error handling for OpenAI
func TestGenerateContentREST_OpenAI_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error": {"message": "Insufficient quota"}}`, http.StatusTooManyRequests)
	}))
	defer server.Close()

	originalEndpoint := openAIAPIEndpoint
	openAIAPIEndpoint = server.URL
	defer func() { openAIAPIEndpoint = originalEndpoint }()

	os.Setenv("OPENAI_API_KEY", "test_openai_key")
	defer os.Unsetenv("OPENAI_API_KEY")

	input := LLMInput{Scenario: "OpenAI Error Test"}
	_, err := GenerateContentREST_open(input)

	if err == nil {
		t.Fatal("GenerateContentREST_open expected error, got nil")
	}
	if !strings.Contains(err.Error(), "API error: status 429") {
		t.Errorf("Expected error to contain 'API error: status 429', got: %v", err)
	}
}

// TestGenerateContentREST_OpenAI_MalformedResponse tests malformed JSON in LLMOutput for OpenAI
func TestGenerateContentREST_OpenAI_MalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Valid ChatCompletionResponse, but invalid LLMOutput JSON in Content
		apiResp := ChatCompletionResponse{
			Choices: []struct {
				Message ChatMessage `json:"message"`
			}{
				{
					Message: ChatMessage{
						Role:    "assistant",
						Content: `{"next_message": "Valid message", "fulfilled": "not_a_boolean"}`, // Malformed LLMOutput
					},
				},
			},
		}
		json.NewEncoder(w).Encode(apiResp)
	}))
	defer server.Close()

	originalEndpoint := openAIAPIEndpoint
	openAIAPIEndpoint = server.URL
	defer func() { openAIAPIEndpoint = originalEndpoint }()

	os.Setenv("OPENAI_API_KEY", "test_openai_key")
	defer os.Unsetenv("OPENAI_API_KEY")

	input := LLMInput{Scenario: "OpenAI Malformed Test"}
	_, err := GenerateContentREST_open(input)

	if err == nil {
		t.Fatal("GenerateContentREST_open expected error for malformed LLMOutput JSON, got nil")
	}
	if !strings.Contains(err.Error(), "failed to unmarshal LLMOutput") {
		t.Errorf("Expected error to contain 'failed to unmarshal LLMOutput', got: %v", err)
	}
}

// TestGenerateContentREST_OpenAI_NoAPIKey tests missing API key for OpenAI
func TestGenerateContentREST_OpenAI_NoAPIKey(t *testing.T) {
	originalApiKey := os.Getenv("OPENAI_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	defer os.Setenv("OPENAI_API_KEY", originalApiKey)

	input := LLMInput{Scenario: "OpenAI No Key Test"}
	_, err := GenerateContentREST_open(input)

	if err == nil {
		t.Fatal("GenerateContentREST_open expected error when API key is missing, got nil")
	}
	if !strings.Contains(err.Error(), "OPENAI_API_KEY environment variable not set") {
		t.Errorf("Expected error about missing API key, got: %v", err)
	}
}

// TestGenerateContentREST_OpenAI_Timeout tests timeout handling for OpenAI
func TestGenerateContentREST_OpenAI_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a server that closes the connection prematurely to cause a client-side error
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
			return
		}
		conn, _, errHijack := hj.Hijack()
		if errHijack != nil {
			http.Error(w, errHijack.Error(), http.StatusInternalServerError)
			return
		}
		conn.Close() // Close immediately
	}))
	defer server.Close()

	originalEndpoint := openAIAPIEndpoint
	openAIAPIEndpoint = server.URL
	defer func() { openAIAPIEndpoint = originalEndpoint }()

	os.Setenv("OPENAI_API_KEY", "test_openai_key")
	defer os.Unsetenv("OPENAI_API_KEY")

	input := LLMInput{Scenario: "OpenAI Timeout Test"}
	_, err := GenerateContentREST_open(input)

	if err == nil {
		t.Fatalf("Expected an error for request timeout/failure, but got nil")
	}
	t.Logf("Got expected error for timeout/failure: %v", err)
	// Error message depends on how the client handles immediate close.
	// Could be "EOF", "connection reset by peer", or the retry wrapper.
	if !strings.Contains(err.Error(), "request failed after") && !strings.Contains(err.Error(), "context done during request/retry") {
		t.Errorf("Expected error to mention request failure or context issue, got: %s", err.Error())
	}
}

// TestGenerateContentREST_OpenAI_NoChoices tests response with no choices for OpenAI
func TestGenerateContentREST_OpenAI_NoChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		apiResp := ChatCompletionResponse{
			Choices: []struct { // Empty choices
				Message ChatMessage `json:"message"`
			}{},
		}
		json.NewEncoder(w).Encode(apiResp)
	}))
	defer server.Close()

	originalEndpoint := openAIAPIEndpoint
	openAIAPIEndpoint = server.URL
	defer func() { openAIAPIEndpoint = originalEndpoint }()

	os.Setenv("OPENAI_API_KEY", "test_openai_key")
	defer os.Unsetenv("OPENAI_API_KEY")

	input := LLMInput{Scenario: "OpenAI No Choices Test"}
	_, err := GenerateContentREST_open(input)

	if err == nil {
		t.Fatal("GenerateContentREST_open expected error for no choices, got nil")
	}
	if !strings.Contains(err.Error(), "no choices returned from API") {
		t.Errorf("Expected error to contain 'no choices returned from API', got: %v", err)
	}
}

// Helper function to marshal JSON for OpenAI tests, panicking on error for test simplicity
func marshalJSONToStringForOpenAI(v interface{}) string {
	bytes, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal JSON in OpenAI test: %v", err))
	}
	return string(bytes)
}
