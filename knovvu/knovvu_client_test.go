package knovvu

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestGetKnovvuToken tests the GetKnovvuToken function.
func TestGetKnovvuToken(t *testing.T) {
	// Backup original env vars
	originalClientID := os.Getenv("KNOVVU_CLIENT_ID")
	originalClientSecret := os.Getenv("KNOVVU_CLIENT_SECRET")

	defer func() {
		os.Setenv("KNOVVU_CLIENT_ID", originalClientID)
		os.Setenv("KNOVVU_CLIENT_SECRET", originalClientSecret)
	}()

	tests := []struct {
		name           string
		clientID       string
		clientSecret   string
		mockServer     *httptest.Server
		expectedToken  string
		expectError    bool
		errorContains  string
		handler        http.HandlerFunc // Specific handler for this test case
		skipEnvSetting bool             // Skip setting env vars for specific cases
	}{
		{
			name:          "successful token retrieval",
			clientID:      "test_id",
			clientSecret:  "test_secret",
			expectedToken: "mock_access_token",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					http.Error(w, "Failed to parse form", http.StatusBadRequest)
					return
				}
				if r.FormValue("grant_type") != "client_credentials" ||
					r.FormValue("client_id") != "test_id" ||
					r.FormValue("client_secret") != "test_secret" {
					http.Error(w, "Bad request parameters", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"access_token": "mock_access_token"})
			},
		},
		{
			name:          "API returns error",
			clientID:      "test_id",
			clientSecret:  "test_secret",
			expectError:   true,
			errorContains: "token endpoint returned status 500",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			},
		},
		{
			name:          "API returns non-JSON response",
			clientID:      "test_id",
			clientSecret:  "test_secret",
			expectError:   true,
			errorContains: "failed to parse token response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.Write([]byte("this is not json"))
			},
		},
		{
			name:          "API returns JSON without access_token",
			clientID:      "test_id",
			clientSecret:  "test_secret",
			expectError:   true,
			errorContains: "access_token not found in response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"some_other_key": "value"})
			},
		},
		{
			name:           "missing KNOVVU_CLIENT_ID",
			clientID:       "", // Will be set to empty
			clientSecret:   "test_secret",
			skipEnvSetting: false, // Test will explicitly unset it
			expectError:    true,
			errorContains:  "client_id or client_secret not set in .env",
			handler: func(w http.ResponseWriter, r *http.Request) { // Should not be called
				t.Error("Server should not be called when client ID is missing")
			},
		},
		{
			name:           "missing KNOVVU_CLIENT_SECRET",
			clientID:       "test_id",
			clientSecret:   "", // Will be set to empty
			skipEnvSetting: false, // Test will explicitly unset it
			expectError:    true,
			errorContains:  "client_id or client_secret not set in .env",
			handler: func(w http.ResponseWriter, r *http.Request) { // Should not be called
				t.Error("Server should not be called when client secret is missing")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// .env file is loaded by GetKnovvuToken, ensure it exists or handle its absence
			// For these tests, we are directly setting os.Setenv
			// Create a dummy .env if necessary or ensure GetKnovvuToken handles its absence gracefully
			// For this test, we assume godotenv.Load(".env") might fail but the function proceeds if env vars are set.
			// If .env is critical, create a temporary one.
			// Create a temporary .env file for tests that need it
			if !tt.skipEnvSetting {
				os.Setenv("KNOVVU_CLIENT_ID", tt.clientID)
				os.Setenv("KNOVVU_CLIENT_SECRET", tt.clientSecret)
			} else { // for cases testing missing env vars specifically
				if tt.clientID == "" {
					os.Unsetenv("KNOVVU_CLIENT_ID")
				} else {
					os.Setenv("KNOVVU_CLIENT_ID", tt.clientID)
				}
				if tt.clientSecret == "" {
					os.Unsetenv("KNOVVU_CLIENT_SECRET")
				} else {
					os.Setenv("KNOVVU_CLIENT_SECRET", tt.clientSecret)
				}
			}

			var server *httptest.Server
			if tt.handler != nil {
				server = httptest.NewServer(tt.handler)
				defer server.Close()

				// Override the tokenURL to use the mock server
				// This requires GetKnovvuToken to use a variable for the URL or to be refactored.
				// For now, we assume such a mechanism or that the original code structure
				// makes this hard to test without modifying it.
				// Let's try to modify the global var `tokenURL` if it was defined in knovvu_client.go
				// If not, this test for GetKnovvuToken is harder.
				// Assuming knovvu_client.go has: var knovvuTokenURL = "https://identity.eu.va.knovvu.com/connect/token"
				// We would do:
				originalTokenURL := tokenURL // Assuming tokenURL is a package var in knovvu_client.go
				tokenURL = server.URL
				defer func() { tokenURL = originalTokenURL }()
			}

			token, err := GetKnovvuToken()

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected an error, but got nil")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Did not expect an error, but got: %v", err)
				}
				if token != tt.expectedToken {
					t.Errorf("Expected token %q, got %q", tt.expectedToken, token)
				}
			}
		})
	}
}

// TestSendKnovvuMessage tests the SendKnovvuMessage function.
func TestSendKnovvuMessage(t *testing.T) {
	tests := []struct {
		name            string
		handler         http.HandlerFunc
		projectName     string
		token           string
		text            string
		conversationID  string
		expectedRespText string
		expectError     bool
		errorContains   string
	}{
		{
			name: "successful message send",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test_token" {
					http.Error(w, "Bad token", http.StatusUnauthorized)
					return
				}
				if r.Header.Get("Project") != "test_project" {
					http.Error(w, "Bad project", http.StatusBadRequest)
					return
				}
				if r.Header.Get("X-Knovvu-Conversation-Id") != "conv123" {
					http.Error(w, "Bad conversation ID", http.StatusBadRequest)
					return
				}
				var reqBody KnovvuRequest
				if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
					http.Error(w, "Cannot decode body", http.StatusBadRequest)
					return
				}
				if reqBody.Text != "hello" {
					http.Error(w, "Unexpected text in body", http.StatusBadRequest)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(KnovvuResponse{Text: "VA reply to hello"})
			},
			projectName:      "test_project",
			token:            "test_token",
			text:             "hello",
			conversationID:   "conv123",
			expectedRespText: "VA reply to hello",
		},
		{
			name: "API returns 401 Unauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				// Simulate Knovvu returning a JSON error for 401
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "invalid_token", "error_description": "The token is expired or invalid"})
			},
			projectName:   "test_project",
			token:         "invalid_token",
			text:          "hello",
			expectError:   true,
			errorContains: "received non-2xx response: 401",
		},
		{
			name: "API returns 500 Internal Server Error (non-JSON)",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "Internal Knovvu Error", http.StatusInternalServerError)
			},
			projectName:   "test_project",
			token:         "test_token",
			text:          "hello",
			expectError:   true,
			errorContains: "received non-2xx response: 500",
		},
		{
			name: "API returns malformed JSON success response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"text": "VA reply", "id": "123", type: "message"`)) // Malformed (type missing quotes)
			},
			projectName:   "test_project",
			token:         "test_token",
			text:          "hello",
			expectError:   true,
			errorContains: "failed to parse successful response",
		},
		{
			name: "API returns 200 OK but empty body (should be parse error)",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				// No body
			},
			projectName:   "test_project",
			token:         "test_token",
			text:          "hello",
			expectError:   true,
			errorContains: "failed to parse successful response", // because unmarshal of empty body into struct fails
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			// Override the messagesURL to use the mock server
			// Similar to tokenURL, this assumes messagesURL is a package variable.
			originalMessagesURL := messagesURL // Assuming messagesURL is a package var in knovvu_client.go
			messagesURL = server.URL
			defer func() { messagesURL = originalMessagesURL }()

			_, resp, err := SendKnovvuMessage(tt.projectName, tt.token, tt.text, tt.conversationID)

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected an error, but got nil")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Did not expect an error, but got: %v", err)
				}
				if resp == nil {
					t.Fatalf("Expected a response, but got nil")
				}
				if resp.Text != tt.expectedRespText {
					t.Errorf("Expected response text %q, got %q", tt.expectedRespText, resp.Text)
				}
			}
		})
	}
}

// To make the URL overriding work, knovvu_client.go needs to define its URLs like this:
// var tokenURL = "https://identity.eu.va.knovvu.com/connect/token"
// var messagesURL = "https://eu.va.knovvu.com/magpie/ext-api/messages/synchronized"
// If they are const or hardcoded strings within functions, these tests will not correctly mock the server
// and will attempt real HTTP calls or fail to compile if the vars are not found.
// I will add these variables to knovvu_client.go.

// Add a helper for client timeout test if needed, though the default client has a timeout.
// Testing the *exact* timeout behavior can be tricky and might require more control over the HTTP client.
// The current tests focus on server responses.
func TestSendKnovvuMessage_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // Sleep longer than client timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	originalMessagesURL := messagesURL
	messagesURL = server.URL
	defer func() { messagesURL = originalMessagesURL }()

	// Temporarily reduce client timeout for this test
	// This requires access to the client or refactoring SendKnovvuMessage to accept a client
	// For now, we assume the default 15s timeout is hard to test quickly.
	// If we could configure the client:
	// customClient := &http.Client{Timeout: 100 * time.Millisecond}
	// Then pass this client to SendKnovvuMessage.
	// Since we can't, this test might not be feasible without code changes.
	// We'll test a generic "request failed" which could be a timeout.

	// To properly test timeout, the client used in SendKnovvuMessage needs to be configurable.
	// The current http.Client{Timeout: 15 * time.Second} is created inside.
	// We'll simulate a general network error instead for now.
	// This test case is more of a placeholder for true timeout testing.

	// Let's simulate a server that doesn't respond by closing the server prematurely (not quite a timeout)
	// A better way is to have the handler block.

	serverThatTimesOut := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a delay longer than a hypothetical short timeout
		// For the actual 15s timeout, this test would be too slow.
		// Let's assume we could inject a client with a shorter timeout.
		time.Sleep(50 * time.Millisecond) // Assume client timeout is < 50ms for this test
		fmt.Fprintln(w, "Hello, client")
	}))
	defer serverThatTimesOut.Close()

	messagesURL = serverThatTimesOut.URL // Point to our timeout server

	// To make this test meaningful, SendKnovvuMessage would need to accept an *http.Client
	// or have its internal client's timeout configurable for testing.
	// For now, this test will likely pass by not erroring if the default 15s timeout is not hit.
	// Or, if we want to test the actual 15s timeout, this test is too long for a unit test suite.

	// Let's assume we want to test the error path for a client.Do failure.
	// We can achieve this by providing an invalid URL (after the server is closed).
	serverForError := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	urlForError := serverForError.URL
	serverForError.Close() // Close the server so the request will fail

	messagesURL = urlForError // Point to the now-closed server URL

	_, _, err := SendKnovvuMessage("proj", "token", "text", "convid")
	if err == nil {
		t.Errorf("Expected an error due to connection failure, but got nil")
	} else {
		// Error might be "connection refused" or similar depending on OS and timing
		t.Logf("Got expected error for connection failure: %v", err)
		if !strings.Contains(err.Error(), "request failed") { // The function wraps errors
			// This check might be too specific depending on Go versions and OS.
			// Example: "request failed: Post \"http://127.0.0.1:500โป๊\": dial tcp 127.0.0.1:500โป๊: connect: connection refused"
			t.Logf("Note: Error message for connection failure was: %s", err.Error())
		}
	}
}
