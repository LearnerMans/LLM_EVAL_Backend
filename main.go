package main

import (
	"evaluator/agent"
	"evaluator/db"
	"evaluator/llm"
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
	scenario := "An Arab user asks about Mohap departments and then thanks the bot and finishes the flow. Say yes and take the survey"
	expeted_Outcome := "The bot should answer the question. The Bot shoud ask the user if they want to fill a survey and allow the user to fill survey It might take several tries"
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
	// _, err = testingAgent.Run()
	// if err != nil {
	// 	log.Printf("Error running testing agent: %v", err)
	// }

	// Parallel run with 3 scenarios
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
	results, errs := testingAgent.ParallelRun(scenarios, expectedOutcomes)
	for i, state := range results {
		if errs[i] != nil {
			log.Printf("Scenario %d error: %v", i+1, errs[i])
		} else {
			log.Printf("Scenario %d completed. Fulfilled: %v, Turns: %d", i+1, state.Fulfilled, state.TurnCount)
		}
	}
}
