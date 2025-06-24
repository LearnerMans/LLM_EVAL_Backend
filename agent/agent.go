package agent

import (
	"evaluator/knovvu"
	"evaluator/llm"
	"fmt"
	"log"

	"github.com/google/uuid"
)

// Agent struct holds the state and configuration for a single scenario execution.
type Agent struct {
	Scenario        string
	ExpectedOutcome string
	State           llm.CurrentState
	Project         string
}

// NewAgent creates a new agent for a given scenario.
func NewAgent(project, scenario, expectedOutcome string, initialState llm.CurrentState) *Agent {
	return &Agent{
		Scenario:        scenario,
		ExpectedOutcome: expectedOutcome,
		State:           initialState,
		Project:         project,
	}
}

// Run executes the agent's main loop until the scenario is fulfilled or max turns are reached.
func (a *Agent) Run() (*llm.CurrentState, error) {
	fmt.Printf("--- Starting Scenario: %s ---\n", a.Scenario)

	knovvuToken, err := knovvu.GetKnovvuToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get Knovvu token: %w", err)
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

		llmResponse, err := llm.GenerateContentREST(llmInput)
		if err != nil {
			return nil, fmt.Errorf("failed to generate content from LLM: %w", err)
		}

		// Update agent's fulfilled status from LLM response
		a.State.Fulfilled = llmResponse.Fulfilled

		// Log the LLM's reasoning
		log.Printf("LLM Reasoning: %s\n", llmResponse.Reasoning)
		log.Printf("LLM Strategy: %s\n", llmResponse.Strategy)

		// 2. Send the message to Knovvu VA
		userMessage := llmResponse.NextMessage
		fmt.Printf("Sending to VA: %s\n", userMessage)

		_, knovvuResp, err := knovvu.SendKnovvuMessage(a.Project, knovvuToken, userMessage, conversationID)
		// Even if an error occurs, we might have a partial response or want to log specific details.
		// However, the primary concern is a nil pointer dereference if knovvuResp is nil.
		if err != nil {
			// Log the error and decide if we should continue or return.
			// For now, we'll return, but in a more complex scenario, we might try to recover or use a default VA response.
			log.Printf("Error sending message to Knovvu: %v. Knovvu response object: %+v", err, knovvuResp)
			return nil, fmt.Errorf("failed to send message to Knovvu: %w", err)
		}

		vaResponse := "No response text found."
		if knovvuResp != nil && knovvuResp.Text != "" { // Added check for non-empty Text
			vaResponse = knovvuResp.Text
		} else if knovvuResp != nil {
			log.Printf("Knovvu response was not nil, but Text field was empty. Full response: %+v", knovvuResp)
			// vaResponse remains "No response text found."
		} else {
			log.Println("Knovvu response was nil.")
			// vaResponse remains "No response text found."
		}
		fmt.Printf("Received from VA: %s\n", vaResponse)

		// 3. Update the history
		a.State.History = append(a.State.History, llm.HistoryItem{
			Turn:      a.State.TurnCount,
			User:      userMessage,
			Assistant: vaResponse,
		})

		// 4. Check for fulfillment to break the loop
		if a.State.Fulfilled {
			fmt.Println("\n--- Scenario Fulfilled ---")
			break
		}
	}

	if !a.State.Fulfilled {
		fmt.Println("\n--- Max turns reached ---")
	}

	return &a.State, nil
}
