package main

import (
	"evaluator/agent"
	"evaluator/db"
	"evaluator/llm"
	"fmt"
	"log"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	err := godotenv.Load(".env")
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
		log.Println("Continuing with environment variables that might be set in the system")
	}

	// Establish DB connection
	dbConn, err := db.ConnectDB()
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer dbConn.Close()

	// //CreateDB
	// db.InitDB()

	//Create new Testing agent
	scenario := "A user will keep on asking questions related to the ceo of different companies and will not ask about the services that the bot offers"
	expeted_Outcome := "the bot will end the conversation after 3 attempts"
	initialState := llm.CurrentState{
		History:   []llm.HistoryItem{},
		TurnCount: 0,
		MaxTurns:  5,
		Fulfilled: false,
	}
	llmClient, err := llm.NewLLMClient(llm.CohereProvider, llm.CohereModel)
	if err != nil {
		log.Fatalf("Error creating LLM client: %v", err)
	}
	testingAgent := agent.NewAgent("MOHAP-BOT", scenario, expeted_Outcome, initialState, llmClient, dbConn)
	//Run the testing agent
	_, err = testingAgent.Run()
	if err != nil {
		log.Fatalf("Error running testing agent: %v", err)
	}
	//Print the testing agent state
	fmt.Println("Testing agent state:")
	fmt.Println(testingAgent.State)
	//Print the testing agent state history
	fmt.Println("Testing agent state history:")
	fmt.Println(testingAgent.State.History)

}
