package agent

import (
	"database/sql"
	"errors"
	"evaluator/knovvu"
	"evaluator/llm"
	"evaluator/repository"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
)

var ErrInternal = errors.New("failed to send message to Knovvu")

// Agent struct holds the state and configuration for a single scenario execution.
type Agent struct {
	Scenario        string
	ExpectedOutcome string
	State           llm.CurrentState
	Project         string
	LLM             llm.LLM
	DB              *sql.DB
	Store           repository.Store
}

// NewAgent creates a new agent for a given scenario.
func NewAgent(project, scenario, expectedOutcome string, initialState llm.CurrentState, llm llm.LLM, db *sql.DB) *Agent {
	return &Agent{
		Scenario:        scenario,
		ExpectedOutcome: expectedOutcome,
		State:           initialState,
		Project:         project,
		LLM:             llm,
		DB:              db,
		Store:           *repository.NewStore(db),
	}
}

// Helper function to extract quick replies from Knovvu hero card attachments
func extractQuickRepliesFromAttachments(attachments []interface{}) string {
	for _, att := range attachments {
		attMap, ok := att.(map[string]interface{})
		if !ok {
			continue
		}
		if attMap["contentType"] == "application/vnd.microsoft.card.hero" {
			content, ok := attMap["content"].(map[string]interface{})
			if !ok {
				continue
			}
			promptText, _ := content["text"].(string)
			buttons, ok := content["buttons"].([]interface{})
			if !ok || len(buttons) == 0 {
				continue
			}
			var quickReplies []string
			for _, btn := range buttons {
				btnMap, ok := btn.(map[string]interface{})
				if !ok {
					continue
				}
				title, _ := btnMap["title"].(string)
				value, _ := btnMap["value"].(string)
				quickReplies = append(quickReplies, "  - "+title+" (value: "+value+")")
			}
			result := ""
			if promptText != "" {
				result += "Prompt: " + promptText + "\n"
			}
			result += "Options:\n" + strings.Join(quickReplies, "\n")
			return result
		}
	}
	return ""
}

// Run executes the agent's main loop until the scenario is fulfilled or max turns are reached.
// If an error occurs, it is returned and should be handled by the caller (never causes server exit).
func (a *Agent) Run() (*llm.CurrentState, *llm.JudgmentResult, error) {
	fmt.Printf("--- Starting Scenario: %s ---\n", a.Scenario)

	knovvuToken, err := knovvu.GetKnovvuToken()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get Knovvu token: %w", err)
	}
	conversationID := uuid.New().String()

	for a.State.TurnCount < a.State.MaxTurns && !a.State.Fulfilled {
		a.State.TurnCount++
		fmt.Printf("\n--- Turn %d ---\n", a.State.TurnCount)

		// 1. Generate the next message using the LLM
		llmInput := llm.LLMInput{
			Scenario:        a.Scenario,
			ExpectedOutcome: a.ExpectedOutcome,
			CurrentState:    a.State,
			Version:         "2.0",
		}

		llmResponse, err := a.LLM.GenerateContentREST(llm.SystemPrompt, llmInput)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate content from LLM: %w", err)
		}

		// Update agent's fulfilled status from LLM response
		a.State.Fulfilled = llmResponse.Fulfilled

		// Log the LLM's reasoning
		log.Printf("LLM Reasoning: %s\n", llmResponse.Reasoning)
		log.Printf("LLM Strategy: %s\n", llmResponse.Strategy)
		log.Printf("Is Fullfilled: %v\\n", a.State.Fulfilled)

		// 2. Send the message to Knovvu VA
		userMessage := llmResponse.NextMessage
		fmt.Printf("Sending to VA: %s\n", userMessage)
		if userMessage != "" {

			_, knovvuResp, err := knovvu.SendKnovvuMessage(a.Project, knovvuToken, userMessage, conversationID)
			if err != nil {
				fmt.Printf("failed to send message to Knovvu: %v", err)
				return nil, nil, ErrInternal
			}

			vaResponse := "No response text found."
			if knovvuResp != nil {
				if knovvuResp.Text != "" {
					vaResponse = knovvuResp.Text
				} else if len(knovvuResp.Attachments) > 0 {
					quickReplies := extractQuickRepliesFromAttachments(knovvuResp.Attachments)
					if quickReplies != "" {
						vaResponse = quickReplies
					}
				}
			}
			fmt.Printf("Received from VA: %s\n", vaResponse)
			// 3. Update the history
			a.State.History = append(a.State.History, llm.HistoryItem{
				Turn:      a.State.TurnCount,
				User:      userMessage,
				Assistant: vaResponse,
			})

		}

		// 4. Check for fulfillment to break the loop
		if a.State.Fulfilled {
			fmt.Println("\n--- Scenario Fulfilled ---")
			break
		}
	}
	judgeInput := llm.JudgeInput{Scenario: a.Scenario, Conversation: a.State.History}
	judgeReslts, err := a.LLM.GenerateJudgmentREST(llm.JudgePrompt, judgeInput)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate Judgement Results from LLM: %w", err)
	}

	fmt.Printf("Judgement is %s\n", judgeReslts.Judgement)
	fmt.Printf("Confidence is %s\n", judgeReslts.Confidence)
	fmt.Printf("Scenario Completion Score is %v\n", judgeReslts.ScenarioCompletionScore)
	fmt.Printf("Conversation Quality Score is %v\n", judgeReslts.ConversationQualityScore)
	fmt.Printf("Evidence Summary %v\n\n", judgeReslts.EvidenceSummary)
	if !a.State.Fulfilled {
		fmt.Println("\n--- Max turns reached ---")
	}

	return &a.State, judgeReslts, nil
}

// ParallelRun runs up to 5 scenarios in parallel at a time, each with its expected outcome.
func (a *Agent) ParallelRun(scenarios []string, expectedOutcomes []string) ([]*llm.CurrentState, []error) {
	if len(scenarios) != len(expectedOutcomes) {
		return nil, []error{fmt.Errorf("scenarios and expectedOutcomes must have the same length")}
	}
	type result struct {
		idx   int
		state *llm.CurrentState
		err   error
	}

	maxParallel := 5
	results := make([]*llm.CurrentState, len(scenarios))
	errs := make([]error, len(scenarios))
	jobs := make(chan int, len(scenarios))
	resCh := make(chan result, len(scenarios))

	// Worker function
	worker := func() {
		for idx := range jobs {
			initState := llm.CurrentState{
				History:   []llm.HistoryItem{},
				TurnCount: 0,
				MaxTurns:  10,
				Fulfilled: false,
			}
			subAgent := NewAgent(a.Project, scenarios[idx], expectedOutcomes[idx], initState, a.LLM, a.DB)
			//TODO: add judgment result to the state
			state, _, err := subAgent.Run()
			resCh <- result{idx: idx, state: state, err: err}
		}
	}

	// Start workers
	for i := 0; i < maxParallel; i++ {
		go worker()
	}

	// Send jobs
	for i := range scenarios {
		jobs <- i
	}
	close(jobs)

	// Collect results
	for i := 0; i < len(scenarios); i++ {
		r := <-resCh
		results[r.idx] = r.state
		errs[r.idx] = r.err
	}

	return results, errs
}
