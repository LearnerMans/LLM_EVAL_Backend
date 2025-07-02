# LLM Evaluation Server

## Overview

The LLM Evaluation Server is a Go-based application designed to test and evaluate conversational AI systems. It implements a ReAct (Reasoning + Acting) agent pattern that simulates user interactions with a Virtual Assistant (VA) to evaluate its performance against predefined scenarios.

The system can utilize various Large Language Models (LLMs) like Google's Gemini, Cohere, or OpenAI's models to generate intelligent, context-aware messages. These messages are sent to a Knovvu Virtual Assistant. The agent continues the conversation loop until either:
1. The scenario is fulfilled (as determined by the LLM)
2. The maximum number of turns is reached.
Finally, the LLM provides a judgment on the conversation quality and scenario completion.

## Architecture

The project follows a modular architecture with the following components:

- **Agent Module**: Implements the core ReAct loop, manages conversation flow, and supports parallel scenario execution.
- **LLM Module**: Handles communication with various LLM APIs (Gemini, Cohere, OpenAI) for generating intelligent responses and final judgments.
- **Knovvu Module**: Manages communication with the Knovvu Virtual Assistant API.
- **Database Module**: Provides storage capabilities for test scenarios, interaction logs, and test runs using SQLite. It utilizes a repository pattern for database interactions.
- **Repository Module**: Abstract database operations for tests, scenarios, test runs, and interactions.

For a visual representation of internal dependencies, please refer to `dependencies.md`.

## Key Features

- **Autonomous Testing**: The agent makes independent decisions about what to say and how to respond.
- **Multi-LLM Support**: Supports various LLM providers (Gemini, Cohere, OpenAI).
- **Scenario-based Evaluation**: Tests are defined as scenarios with expected outcomes.
- **Conversation History Tracking**: Maintains the full history of interactions.
- **Fulfillment Detection**: Automatically determines when a scenario has been successfully completed during the conversation.
- **Post-Conversation Judgment**: LLM provides a final judgment on scenario completion and conversation quality.
- **Detailed Logging**: Records reasoning, strategy, and confidence for each interaction.
- **Parallel Scenario Execution**: Capable of running multiple test scenarios concurrently.
- **Database Integration**: Stores test data, results, and interaction logs in an SQLite database.

## Setup

1. Create a `.env` file in the server directory with the following variables. Obtain the necessary API keys from the respective providers.

```
# Knovvu API credentials
KNOVVU_CLIENT_ID=your_knovvu_client_id_here
KNOVVU_CLIENT_SECRET=your_knovvu_client_secret_here

# Cohere API credentials (Default LLM)
COHERE_API_KEY=your_cohere_api_key_here

# Optional: Gemini API credentials (if using Gemini)
# GEMINI_API_KEY=your_gemini_api_key_here

# Optional: OpenAI API credentials (if using OpenAI)
# OPENAI_API_KEY=your_openai_api_key_here
```

2. Install the required dependencies:

```bash
go mod tidy
```

## Running the Application

To run the application with the default test scenario (which uses Cohere LLM by default):

```bash
go run main.go
```

The application will:
1. Initialize the agent with a test scenario and a specified LLM client.
2. Execute the ReAct loop, generating messages with the chosen LLM and sending them to Knovvu VA.
3. Track and display the conversation history and fulfillment status.
4. After the loop, obtain and display a final judgment from the LLM about the scenario.

## Database Integration

The system includes SQLite database integration for storing:
- Test definitions
- Scenarios
- Test runs
- Interaction histories

The database schema is defined in `db/db.go`. To connect to the database, the application uses `db.ConnectDB()`.
The `db.InitDB()` function can be used to create the database and tables if they don't exist, though it's currently commented out in `main.go` in favor of assuming the DB file `db.db` exists or will be created by `sql.Open`.

The `repository/` directory contains the store and repository implementations for interacting with the database tables.

## Workflow

The core workflow follows this pattern:

1. **Environment Setup**: Load API keys and other configurations from `.env`.
2. **Database Connection**: Establish a connection to the SQLite database.
3. **LLM Client Initialization**: Create an LLM client for the desired provider (e.g., Cohere, Gemini, OpenAI).
4. **Agent Initialization**: The agent is initialized with a project name, scenario, expected outcome, initial state, the LLM client, and the database connection.
5. **Test Execution Started**: The agent begins the ReAct loop for the given scenario.
    a. **Load Test & Scenarios**: Test parameters are loaded (currently from code, potentially from the database in future enhancements).
    b. **Initialize Evaluator State**: The conversation state is prepared (history, turn count, etc.).
    c. **Evaluator Agent Analysis**: The LLM analyzes the current state and generates the next message, reasoning, and strategy.
    d. **Send Message to VA**: The generated message is sent to the Knovvu Virtual Assistant.
    e. **Receive VA Response**: The VA's response is captured.
    f. **Update State & Assess Fulfillment**: The conversation history is updated, and the LLM's assessment of scenario fulfillment is recorded.
    g. **Decision Point**: If the scenario is fulfilled (according to the LLM during the conversation) or max turns are reached, the loop ends; otherwise, it continues to the next turn.
6. **Post-Conversation Judgment**: After the loop, the LLM generates a final judgment on the scenario's success, conversation quality, and provides an evidence summary.
7. **Logging**: Throughout the process, relevant information (LLM reasoning, VA responses, errors) is logged.

## Customizing Scenarios and LLM Provider

To create custom test scenarios and choose an LLM provider, modify the `main.go` file:

```go
package main

import (
	"evaluator/agent"
	"evaluator/db"
	"evaluator/llm"
	"log"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	err := godotenv.Load(".env")
	// ... (error handling) ...

	// Establish DB connection
	dbConn, err := db.ConnectDB()
	// ... (error handling) ...
	defer dbConn.Close()

	// Define scenario details
	scenario := "An user asks about Mohap departments and then thanks the bot and finishes the flow. Say yes and take the survey"
	expectedOutcome := "The bot should answer the question. The Bot shoud ask the user if they want to fill a survey and allow the user to fill survey It might take several tries"
	initialState := llm.CurrentState{
		History:   []llm.HistoryItem{},
		TurnCount: 0,
		MaxTurns:  5, // Adjust as needed
		Fulfilled: false,
	}

	// Initialize LLM client (choose one)
	// llmClient, err := llm.NewLLMClient(llm.GeminiProvider, llm.GeminiPro)
	llmClient, err := llm.NewLLMClient(llm.CohereProvider, llm.CohereModel)
	// llmClient, err := llm.NewLLMClient(llm.OpenAIProvider, llm.OpenAIGPT35Turbo)
	if err != nil {
		log.Fatalf("Error creating LLM client: %v", err)
	}

	// Create new Testing agent
	testingAgent := agent.NewAgent("MOHAP-BOT", scenario, expectedOutcome, initialState, llmClient, dbConn)

	// Run the testing agent for a single scenario
	_, err = testingAgent.Run()
	if err != nil {
		log.Printf("Error running testing agent: %v", err)
	}
}
```

## Running Scenarios in Parallel

The agent supports running multiple scenarios in parallel. Modify `main.go` to use the `ParallelRun` method:

```go
// ... (initial setup as above) ...

	// Initialize LLM client (as above)
	llmClient, err := llm.NewLLMClient(llm.CohereProvider, llm.CohereModel)
	if err != nil {
		log.Fatalf("Error creating LLM client: %v", err)
	}

	// Create a base agent instance (project, LLM, DB are shared)
	baseAgent := agent.NewAgent("MOHAP-BOT", "", "", llm.CurrentState{}, llmClient, dbConn)

	// Define multiple scenarios and their expected outcomes
	scenarios := []string{
		"An Arab user asks about Mohap departments and then thanks the bot and finishes the flow. Say yes and take the survey",
		"A user asks for the location of the nearest Mohap hospital and then requests directions.",
		"A user wants to book an appointment with a Mohap doctor and then cancels it.",
	}
	expectedOutcomes := []string{
		"The bot should answer the question. The Bot shoud ask the user if they want to fill a survey and allow the user to fill survey It might take several tries",
		"The bot should provide the location and then give directions to the hospital.",
		"The bot should help the user book an appointment and then process the cancellation request.",
	}

	// Run scenarios in parallel
	// Note: MaxTurns for parallel runs is set within the ParallelRun method (currently 10).
	// The initialState for each sub-agent in ParallelRun is created fresh.
	results, errs := baseAgent.ParallelRun(scenarios, expectedOutcomes)

	for i, state := range results {
		if errs[i] != nil {
			log.Printf("Scenario %d error: %v", i+1, errs[i])
		} else if state != nil { // Check if state is nil, which can happen if an error occurred early
			log.Printf("Scenario %d completed. Fulfilled during run: %v, Turns: %d", i+1, state.Fulfilled, state.TurnCount)
			// Further details from the state (like final judgment) would be part of the 'state' object
			// if it were enhanced to include the judgment results directly.
			// Currently, judgment results are printed to console within Run() and not returned directly by ParallelRun's states.
		} else {
			log.Printf("Scenario %d did not complete successfully and state is nil.", i+1)
		}
	}
```

## Troubleshooting

- **Missing Environment Variables**: Ensure your `.env` file is properly configured with the correct API keys for Knovvu and your chosen LLM provider.
- **API Errors**: Check your internet connection and verify API credentials and permissions for the respective services.
- **Timeout Issues**: The system implements some retry logic for Knovvu communication. Check LLM client configurations for specific timeout settings if issues persist.
- **Unexpected Responses**: Check the error logs in the LLM output and the console output for debugging information. The LLM's reasoning and strategy logs can be particularly helpful.
- **Database Issues**: Ensure `db.db` file has write permissions or the directory is writable if the file doesn't exist.