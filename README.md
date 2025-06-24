# LLM Evaluation Server

## Overview

The LLM Evaluation Server is a Go-based application designed to test and evaluate conversational AI systems. It implements a ReAct (Reasoning + Acting) agent pattern that simulates user interactions with a Virtual Assistant (VA) to evaluate its performance against predefined scenarios.

The system uses Google's Gemini LLM to generate intelligent, context-aware messages that are sent to a Knovvu Virtual Assistant. The agent continues the conversation loop until either:
1. The scenario is fulfilled (as determined by the LLM)
2. The maximum number of turns is reached

## Architecture

The project follows a modular architecture with the following components:

- **Agent Module**: Implements the core ReAct loop that manages the conversation flow
- **LLM Module**: Handles communication with the Gemini API for generating intelligent responses
- **Knovvu Module**: Manages communication with the Knovvu Virtual Assistant API
- **Database Module**: Provides storage capabilities for test scenarios and interaction logs

## Key Features

- **Autonomous Testing**: The agent makes independent decisions about what to say and how to respond
- **Scenario-based Evaluation**: Tests are defined as scenarios with expected outcomes
- **Conversation History Tracking**: Maintains the full history of interactions
- **Fulfillment Detection**: Automatically determines when a scenario has been successfully completed
- **Detailed Logging**: Records reasoning, strategy, and confidence for each interaction

## Setup

1. Create a `.env` file in the server directory with the following variables:

```
# Gemini API credentials
GEMINI_API_KEY=your_gemini_api_key_here

# Knovvu API credentials
KNOVVU_CLIENT_ID=your_knovvu_client_id_here
KNOVVU_CLIENT_SECRET=your_knovvu_client_secret_here

# Optional: OpenAI API credentials (if using OpenAI instead of Gemini)
# OPENAI_API_KEY=your_openai_api_key_here
```

2. Install the required dependencies:

```bash
go mod tidy
```

## Running the Application

To run the application with the default test scenario:

```bash
go run main.go
```

The application will:
1. Initialize the agent with a test scenario
2. Execute the ReAct loop, generating messages with Gemini and sending them to Knovvu VA
3. Track and display the conversation history and fulfillment status

## Database Integration

The system includes SQLite database integration for storing:
- Test definitions
- Scenarios
- Test runs
- Interaction histories

To initialize the database:

```go
db.InitDB()
```

## Workflow

The core workflow follows this pattern:

1. **Test Execution Started**: The agent is initialized with a scenario and expected outcome
2. **Load Test & Scenarios**: Test parameters are loaded (from code or database)
3. **Initialize Evaluator State**: The conversation state is prepared
4. **Evaluator Agent Analysis**: The LLM analyzes the current state and generates the next message
5. **Send Message to VA**: The message is sent to the Knovvu Virtual Assistant
6. **Receive VA Response**: The response is captured and stored
7. **Update State & Assess Fulfillment**: The conversation state is updated and checked for completion
8. **Decision Point**: If the scenario is fulfilled or max turns reached, the test ends; otherwise, the loop continues

## Customizing Scenarios

To create custom test scenarios, modify the `main.go` file:

```go
scenario := "Your custom scenario description"
expected_outcome := "The expected outcome description"
initialState := llm.CurrentState{
    History:   []llm.HistoryItem{},
    TurnCount: 0,
    MaxTurns:  5, // Adjust as needed
    Fulfilled: false,
}
testingAgent := agent.NewAgent("PROJECT-NAME", scenario, expected_outcome, initialState)
```

## Troubleshooting

- **Missing Environment Variables**: Ensure your `.env` file is properly configured
- **API Errors**: Check your internet connection and verify API credentials
- **Timeout Issues**: The system implements retry logic for transient errors
- **Unexpected Responses**: Check the error logs in the LLM output for debugging information