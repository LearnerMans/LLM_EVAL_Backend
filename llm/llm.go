package llm

import (
	"fmt"
	"os"
	"time"
)

type LLMProvider string

var (
	OpenAIProvider LLMProvider = "openai"
	GeminiProvider LLMProvider = "gemini"
	CohereProvider LLMProvider = "cohere"
)

var (
	ContextTimeout = 90 * time.Second
)

var (
	OpenAIModel = "gpt-4.1"
	GeminiModel = "gemini-2.5-flash"
	CohereModel = "command-a-03-2025"
)

type LLM interface {
	GenerateContentREST(prompt string, input LLMInput) (*LLMOutput, error)
	GenerateJudgmentREST(judgePrompt string, input JudgeInput) (*JudgmentResult, error)
}

type GeminiClient struct {
	apiKey string
	Model  string
}

type OpenAIClient struct {
	apiKey string
	Model  string
}

type CohereClient struct {
	apiKey string
	Model  string
}

func NewLLMClient(provider LLMProvider, model string) (LLM, error) {
	switch provider {
	case OpenAIProvider:
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY environment variable not set")
		}
		return &OpenAIClient{
			apiKey: apiKey,
			Model:  OpenAIModel,
		}, nil
	case GeminiProvider:
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY environment variable not set")
		}
		return &GeminiClient{
			apiKey: apiKey,
			Model:  GeminiModel,
		}, nil
	case CohereProvider:
		apiKey := os.Getenv("COHERE_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("COHERE_API_KEY environment variable not set")
		}
		return &CohereClient{
			apiKey: apiKey,
			Model:  CohereModel,
		}, nil
	default:
		return nil, fmt.Errorf("invalid provider: %s", provider)
	}
}

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

type JudgeInput struct {
	Scenario     string        `json:"scenario"`
	Conversation []HistoryItem `json:"conversation"`
}

type JudgmentResult struct {
	Judgment                 string  `json:"judgment"`
	Confidence               string  `json:"confidence"`
	EvidenceSummary          string  `json:"evidence_summary"`
	ScenarioCompletionScore  float64 `json:"scenario_completion_score"`
	ConversationQualityScore float64 `json:"conversation_quality_score"`
}

func (c *OpenAIClient) GenerateJudgmentREST(judgePrompt string, input JudgeInput) (*JudgmentResult, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *GeminiClient) GenerateJudgmentREST(judgePrompt string, input JudgeInput) (*JudgmentResult, error) {
	return nil, fmt.Errorf("not implemented")
}
