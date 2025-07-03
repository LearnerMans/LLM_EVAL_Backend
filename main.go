package main

import (
	"encoding/json"
	"evaluator/db"
	repo "evaluator/repository"
	"log"
	"net/http"
	"strconv"

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
	// scenario := "An  user asks about Mohap departments and then thanks the bot and finishes the flow. Say yes and take the survey"
	// expeted_Outcome := "The bot should answer the question. The Bot shoud ask the user if they want to fill a survey and allow the user to fill survey It might take several tries"
	// initialState := llm.CurrentState{
	// 	History:   []llm.HistoryItem{},
	// 	TurnCount: 0,
	// 	MaxTurns:  5,
	// 	Fulfilled: false,
	// }
	// llmClient, err := llm.NewLLMClient(llm.CohereProvider, llm.CohereModel)
	// if err != nil {
	// 	log.Fatalf("Error creating LLM client: %v", err)
	// }
	// testingAgent := agent.NewAgent("MOHAP-BOT", scenario, expeted_Outcome, initialState, llmClient, dbConn)
	// // Run the testing agent
	// _, err = testingAgent.Run()
	// if err != nil {
	// 	log.Printf("Error running testing agent: %v", err)
	// }

	// // Parallel run with 3 scenarios
	// scenarios := []string{
	// 	"An Arab user asks about Mohap departments and then thanks the bot and finishes the flow. Say yes and take the survey",
	// 	"A user asks for the location of the nearest Mohap hospital and then requests directions.",
	// 	"A user wants to book an appointment with a Mohap doctor and then cancels it.",
	// }
	// expectedOutcomes := []string{
	// 	"The bot should answer the question. The Bot shoud ask the user if they want to fill a survey and allow the user to fill survey It might take several tries",
	// 	"The bot should provide the location and then give directions to the hospital.",
	// 	"The bot should help the user book an appointment and then process the cancellation request.",
	// }
	// results, errs := testingAgent.ParallelRun(scenarios, expectedOutcomes)
	// for i, state := range results {
	// 	if errs[i] != nil {
	// 		log.Printf("Scenario %d error: %v", i+1, errs[i])
	// 	} else {
	// 		log.Printf("Scenario %d completed. Fulfilled: %v, Turns: %d", i+1, state.Fulfilled, state.TurnCount)
	// 	}
	// }

	// --- HTTP API Server ---
	dbConn2, err := db.ConnectDB()
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	testRepo := repo.NewTestRepository(dbConn2)
	scenarioRepo := repo.NewScenarioRepository(dbConn2)

	http.HandleFunc("/projects", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == "POST" {
			var newTest repo.Test
			if err := json.NewDecoder(r.Body).Decode(&newTest); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}

			id, err := testRepo.CreateTest(newTest.Name, newTest.TenantID, newTest.ProjectID, newTest.MaxInteractions)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}

			createdTest, err := testRepo.GetTestByID(id)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}

			// Return the full project object, including scenarios (which will be empty)
			project := map[string]interface{}{
				"id":               createdTest.ID,
				"title":            createdTest.Name,
				"tenant_id":        createdTest.TenantID,
				"project_id":       createdTest.ProjectID,
				"max_interactions": createdTest.MaxInteractions,
				"created_at":       createdTest.CreatedAt,
				"scenarios":        []repo.Scenario{},
			}

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(project)
			return
		}

		// Existing GET logic from here
		w.Header().Set("Content-Type", "application/json")
		// For now, return all tests (projects) with their scenarios
		rows, err := dbConn2.Query("SELECT id, name, tenant_id, project_id, max_interactions, created_at FROM tests")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()
		var projects []map[string]interface{}
		for rows.Next() {
			var t repo.Test
			if err := rows.Scan(&t.ID, &t.Name, &t.TenantID, &t.ProjectID, &t.MaxInteractions, &t.CreatedAt); err != nil {
				continue
			}
			scenarios, _ := scenarioRepo.GetScenariosByTestID(t.ID)
			project := map[string]interface{}{
				"id":               t.ID,
				"title":            t.Name,
				"tenant_id":        t.TenantID,
				"project_id":       t.ProjectID,
				"max_interactions": t.MaxInteractions,
				"created_at":       t.CreatedAt,
				"scenarios":        scenarios,
			}
			projects = append(projects, project)
		}
		json.NewEncoder(w).Encode(projects)
	})

	http.HandleFunc("/api/upload-scenarios", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[UPLOAD-SCENARIOS] Received request: method=%s, remote=%s", r.Method, r.RemoteAddr)
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")

		if r.Method == "OPTIONS" {
			log.Printf("[UPLOAD-SCENARIOS] Handled OPTIONS preflight request from %s", r.RemoteAddr)
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != "POST" {
			log.Printf("[UPLOAD-SCENARIOS] Method not allowed: %s from %s", r.Method, r.RemoteAddr)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Method not allowed"})
			return
		}

		type ScenarioUpload struct {
			TestID    string `json:"test_id"`
			Scenarios []struct {
				Description    string `json:"description"`
				ExpectedOutput string `json:"expected_output"`
			} `json:"scenarios"`
		}

		var payload ScenarioUpload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			log.Printf("[UPLOAD-SCENARIOS] Error parsing payload from %s: %v", r.RemoteAddr, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"error from payload parsing": err.Error()})
			return
		}

		testID, err := strconv.Atoi(payload.TestID)
		if err != nil {
			log.Printf("[UPLOAD-SCENARIOS] Invalid test_id value from %s: %v", r.RemoteAddr, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "test_id must be an integer"})
			return
		}

		log.Printf("[UPLOAD-SCENARIOS] Received payload from %s: %+v", r.RemoteAddr, payload)

		results := make([]map[string]interface{}, 0)
		for _, s := range payload.Scenarios {
			log.Printf("[UPLOAD-SCENARIOS] Creating scenario for test_id=%d: description=\"%s\"", testID, s.Description)
			sc, err := scenarioRepo.CreateScenario(testID, s.Description, s.ExpectedOutput)
			if err != nil {
				log.Printf("[UPLOAD-SCENARIOS] Error creating scenario for description=\"%s\": %v", s.Description, err)
				results = append(results, map[string]interface{}{
					"description": s.Description,
					"success":     false,
					"error":       err.Error(),
				})
			} else {
				log.Printf("[UPLOAD-SCENARIOS] Successfully created scenario id=%d for description=\"%s\"", sc.ID, s.Description)
				results = append(results, map[string]interface{}{
					"description": s.Description,
					"success":     true,
					"scenario_id": sc.ID,
				})
			}
		}

		log.Printf("[UPLOAD-SCENARIOS] Sending response to %s: %+v", r.RemoteAddr, results)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"results": results})
	})

	http.HandleFunc("/api/scenarios/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Method not allowed"})
			return
		}

		prefix := "/api/scenarios/"
		testIdStr := r.URL.Path[len(prefix):]
		testId, err := strconv.Atoi(testIdStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid test ID"})
			return
		}

		scenarios, err := scenarioRepo.GetScenariosByTestID(testId)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		out := make([]map[string]interface{}, 0, len(scenarios))
		for _, s := range scenarios {
			out = append(out, map[string]interface{}{
				"id":              s.ID,
				"description":     s.Description,
				"expected_output": s.ExpectedOutput,
				"status":          s.Status,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	})

	log.Println("API server running on :8080 ... (GET, POST /projects)")
	http.ListenAndServe(":8080", nil)
}
