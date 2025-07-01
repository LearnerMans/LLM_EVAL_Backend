package knovvu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type KnovvuRequest struct {
	Text         string            `json:"text"`
	Conversation map[string]string `json:"conversation"`
	ChannelId    string            `json:"channelId"`
	Type         string            `json:"type"`
	Attachments  []any             `json:"attachments"`
	ChannelData  map[string]any    `json:"channelData"`
}

type KnovvuResponse struct {
	Id           string                 `json:"id"`
	Text         string                 `json:"text"`
	Type         string                 `json:"type"`
	Timestamp    string                 `json:"timestamp"`
	Conversation map[string]string      `json:"conversation"`
	From         map[string]string      `json:"from"`
	Recipient    map[string]string      `json:"recipient"`
	ChannelId    string                 `json:"channelId"`
	ChannelData  map[string]interface{} `json:"channelData"`
	Attachments  []interface{}          `json:"attachments"`
}

func GetKnovvuToken() (string, error) {
	err := godotenv.Load(".env")
	if err != nil {
		return "", fmt.Errorf("error loading .env file: %w", err)
	}

	clientID := os.Getenv("KNOVVU_CLIENT_ID")
	clientSecret := os.Getenv("KNOVVU_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("client_id or client_secret not set in .env")
	}

	form := url.Values{}
	form.Add("grant_type", "client_credentials")
	form.Add("client_id", clientID)
	form.Add("client_secret", clientSecret)

	tokenURL := "https://identity.eu.va.knovvu.com/connect/token"
	req, err := http.NewRequest("POST", tokenURL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("access_token not found in response")
	}

	return tokenResp.AccessToken, nil
}

func SendKnovvuMessage(projectName, token, text, conversationID string) ([]byte, *KnovvuResponse, error) {
	url := "https://eu.va.knovvu.com/magpie/ext-api/messages/synchronized"

	text = strings.TrimSpace(text)

	requestBody := KnovvuRequest{
		Text: text,
		Conversation: map[string]string{
			"id": conversationID,
		},
		ChannelId:   "ivr-default",
		Type:        "message",
		Attachments: []interface{}{},
		ChannelData: map[string]any{"responseType": "Text"},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Project", projectName)
	req.Header.Set("X-Knovvu-Conversation-Id", conversationID)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Tenant", "bac")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Println("here reachedd")
		// Attempt to parse error response for more details
		var errorResponse map[string]any
		if err := json.Unmarshal(body, &errorResponse); err == nil {
			// If we can parse the JSON response, include it in the error message
			errorJSON, _ := json.MarshalIndent(errorResponse, "", "  ")
			return body, nil, fmt.Errorf("received non-2xx response: %d\nError details: %s",
				resp.StatusCode, string(errorJSON))
		}

		// If we can't parse the JSON, just return the raw body
		return body, nil, fmt.Errorf("received non-2xx none_parse response: %d\nResponse body: %s",
			resp.StatusCode, string(body))
	}

	// Parse successful response
	var knovvuResp KnovvuResponse
	if err := json.Unmarshal(body, &knovvuResp); err != nil {
		// Print the raw response body as a string
		fmt.Println("Failed to parse response as JSON. Raw response body:")
		fmt.Println(string(body))
		return body, nil, fmt.Errorf("failed to parse successful response: %w\nResponse body: %s",
			err, string(body))
	}

	return body, &knovvuResp, nil
}
