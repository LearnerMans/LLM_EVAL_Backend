package main

import (
	"evaluator/db"
	"evaluator/handlers" // New import
	// "evaluator/agent" - No longer needed here
	// "evaluator/llm" - No longer needed here
	// repo "evaluator/repository" - No longer directly needed for routing setup here
	"log"
	"net/http"
	// "encoding/json" - No longer needed here
	// "strconv" - No longer needed here
	// "strings" - No longer needed here

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
		log.Println("Continuing with environment variables that might be set in the system")
	}

	dbConn, err := db.ConnectDB()
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer dbConn.Close()

	// Initialize the API environment with dependencies
	apiEnv := handlers.NewAPIEnv(dbConn)

	// --- HTTP API Server ---
	// The TestRepo, ScenarioRepo etc. are now initialized within NewAPIEnv and accessed via apiEnv.

	// Handle base /projects path (GET for list, POST for create)
	http.HandleFunc("/projects", apiEnv.ListCreateProjectsHandler)

	// Handle paths under /projects/ (e.g., /projects/{id}, /projects/{id}/run-test)
	// ProjectDispatchHandler will internally route to the correct logic based on path.
	http.HandleFunc("/projects/", apiEnv.ProjectDispatchHandler)

	// Handle /api/upload-scenarios
	http.HandleFunc("/api/upload-scenarios", apiEnv.UploadScenariosHandler)

	// Handle /api/scenarios/{test_id}
	// GetScenariosByTestIDHandler will parse the {test_id} from the path.
	http.HandleFunc("/api/scenarios/", apiEnv.GetScenariosByTestIDHandler)

	// Handle /scenarios/{id}/run
	// ScenarioRunHandler will parse {id} and ensure "/run" suffix.
	http.HandleFunc("/scenarios/", apiEnv.ScenarioRunHandler)


	// --- Logging for registered routes (optional, for verification) ---
	log.Println("Registered route: GET, POST /projects")
	log.Println("Registered route: (various) /projects/*")
	log.Println("Registered route: POST /api/upload-scenarios")
	log.Println("Registered route: GET /api/scenarios/*")
	log.Println("Registered route: POST /scenarios/*/run")


	log.Println("API server running on :8080 ...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
