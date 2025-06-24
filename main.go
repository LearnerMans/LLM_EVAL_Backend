package main

import (
	"fmt"
	"log"

	"github.com/joho/godotenv"

	"evaluator/agent"
	"evaluator/llm"
)

func main() {
	// Load environment variables from .env file
	err := godotenv.Load(".env")
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
		log.Println("Continuing with environment variables that might be set in the system")
	}

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
	testingAgent := agent.NewAgent("MOHAP-BOT", scenario, expeted_Outcome, initialState)
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
