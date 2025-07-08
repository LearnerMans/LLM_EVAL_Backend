package main

import (
	"encoding/json"
	"evaluator/agent"
	"evaluator/db"
	"evaluator/llm"
	repo "evaluator/repository"
	"log"
	"net/http"
	"strconv"
	"strings"

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
		log.Printf("[INFO] /projects endpoint hit: method=%s, remote=%s", r.Method, r.RemoteAddr)
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == "POST" {
			log.Printf("[INFO] Creating new project (POST /projects) from %s", r.RemoteAddr)
			var newTest repo.Test
			if err := json.NewDecoder(r.Body).Decode(&newTest); err != nil {
				log.Printf("[ERROR] Failed to decode new project: %v", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
				return
			}

			id, err := testRepo.CreateTest(newTest.Name, newTest.TenantID, newTest.ProjectID, newTest.MaxInteractions)
			if err != nil {
				log.Printf("[ERROR] Failed to create project: %v", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
				return
			}

			createdTest, err := testRepo.GetTestByID(id)
			if err != nil {
				log.Printf("[ERROR] Failed to fetch created project: %v", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
				return
			}

			log.Printf("[INFO] Project created: id=%d, name=%s", createdTest.ID, createdTest.Name)
			project := map[string]any{
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

		// GET
		log.Printf("[INFO] Listing all projects (GET /projects) from %s", r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		rows, err := dbConn2.Query("SELECT id, name, tenant_id, project_id, max_interactions, created_at FROM tests")
		if err != nil {
			log.Printf("[ERROR] Failed to query projects: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()
		var projects []map[string]any
		for rows.Next() {
			var t repo.Test
			if err := rows.Scan(&t.ID, &t.Name, &t.TenantID, &t.ProjectID, &t.MaxInteractions, &t.CreatedAt); err != nil {
				log.Printf("[ERROR] Failed to scan project row: %v", err)
				continue
			}
			scenarios, _ := scenarioRepo.GetScenariosByTestID(t.ID)
			project := map[string]any{
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
		log.Printf("[INFO] Returned %d projects", len(projects))
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
			json.NewEncoder(w).Encode(map[string]any{"error": "Method not allowed"})
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
			json.NewEncoder(w).Encode(map[string]any{"error from payload parsing": err.Error()})
			return
		}

		testID, err := strconv.Atoi(payload.TestID)
		if err != nil {
			log.Printf("[UPLOAD-SCENARIOS] Invalid test_id value from %s: %v", r.RemoteAddr, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"error": "test_id must be an integer"})
			return
		}

		log.Printf("[UPLOAD-SCENARIOS] Received payload from %s: %+v", r.RemoteAddr, payload)

		results := make([]map[string]any, 0)
		for _, s := range payload.Scenarios {
			log.Printf("[UPLOAD-SCENARIOS] Creating scenario for test_id=%d: description=\"%s\"", testID, s.Description)
			sc, err := scenarioRepo.CreateScenario(testID, s.Description, s.ExpectedOutput)
			if err != nil {
				log.Printf("[UPLOAD-SCENARIOS] Error creating scenario for description=\"%s\": %v", s.Description, err)
				results = append(results, map[string]any{
					"description": s.Description,
					"success":     false,
					"error":       err.Error(),
				})
			} else {
				log.Printf("[UPLOAD-SCENARIOS] Successfully created scenario id=%s for description=\"%s\"", sc.ID, s.Description)
				results = append(results, map[string]any{
					"description": s.Description,
					"success":     true,
					"scenario_id": sc.ID,
				})
			}
		}

		log.Printf("[UPLOAD-SCENARIOS] Sending response to %s: %+v", r.RemoteAddr, results)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"results": results})
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
			json.NewEncoder(w).Encode(map[string]any{"error": "Method not allowed"})
			return
		}

		prefix := "/api/scenarios/"
		testIdStr := r.URL.Path[len(prefix):]
		testId, err := strconv.Atoi(testIdStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"error": "Invalid test ID"})
			return
		}

		scenarios, err := scenarioRepo.GetScenariosByTestID(testId)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}

		out := make([]map[string]any, 0, len(scenarios))
		for _, s := range scenarios {
			out = append(out, map[string]any{
				"id":              s.ID,
				"description":     s.Description,
				"expected_output": s.ExpectedOutput,
				"status":          s.Status,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	})

	// --- Add: Run Test and Test Status Endpoints ---

	// POST /projects/{projectName}/run-test
	http.HandleFunc("/projects/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, PUT, DELETE, OPTIONS")

		log.Printf("[RUN-TEST][BACKEND] Incoming request: method=%s, path=%s, remote=%s", r.Method, r.URL.Path, r.RemoteAddr)

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		path := r.URL.Path
		if strings.HasSuffix(path, "/run-test") {
			if r.Method != "POST" {
				log.Printf("[RUN-TEST][BACKEND] Method not allowed for /run-test: %s", r.Method)
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			prefix := "/projects/"
			suffix := "/run-test"
			projectIDStr := path[len(prefix) : len(path)-len(suffix)]
			projectIDStr = strings.TrimSuffix(projectIDStr, "/")
			if projectIDStr == "" {
				log.Printf("[RUN-TEST][BACKEND][ERROR] Missing project id for run-test")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "Missing project id"})
				return
			}
			log.Printf("[RUN-TEST][BACKEND] Looking up project in DB: id='%s'", projectIDStr)
			projectID, err := strconv.Atoi(projectIDStr)
			if err != nil {
				log.Printf("[RUN-TEST][BACKEND][ERROR] Invalid project id: %s", projectIDStr)
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "Invalid project id"})
				return
			}
			var testID int
			row := dbConn2.QueryRow("SELECT id FROM tests WHERE id = ?", projectID)
			if err := row.Scan(&testID); err != nil {
				log.Printf("[RUN-TEST][BACKEND][ERROR] Project not found for run-test: %d", projectID)
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": "Project not found"})
				return
			}
			log.Printf("[RUN-TEST][BACKEND] Found project: id=%d", testID)
			testRunRepo := repo.NewTestRunRepository(dbConn2)
			runID, _ := testRunRepo.CreateTestRun(testID, nil)
			log.Printf("[RUN-TEST][BACKEND] Test run started: project_id=%d, run_id=%d", testID, runID)
			go func(testID, runID int, projectID int) {
				log.Printf("[RUN-TEST][BACKEND] Running test in background: project_id=%d, run_id=%d", projectID, runID)
				testRunRepo.UpdateTestRunStatus(runID, "running")
				// TODO: Fetch scenarios for this test/project
				// TODO: Run the agent for each scenario (see agent.ParallelRun or similar)
				testRunRepo.UpdateTestRunStatus(runID, "completed")
				log.Printf("[RUN-TEST][BACKEND] Test run completed: project_id=%d, run_id=%d", projectID, runID)
			}(testID, runID, projectID)
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]interface{}{"run_id": runID, "status": "started"})
			return
		} else if strings.HasSuffix(path, "/test-status") {
			if r.Method != "GET" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			prefix := "/projects/"
			suffix := "/test-status"
			projectIDStr := path[len(prefix) : len(path)-len(suffix)]
			projectIDStr = strings.TrimSuffix(projectIDStr, "/")
			log.Printf("[INFO] pRIJECT ıd SENT %v", projectIDStr)
			if projectIDStr == "" {
				log.Printf("[ERROR] Missing project id for test-status")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "Missing project id"})
				return
			}
			log.Printf("[INFO] Getting test status for project id: %s", projectIDStr)
			projectID, err := strconv.Atoi(projectIDStr)
			if err != nil {
				log.Printf("[ERROR] Invalid project id for test-status: %s", projectIDStr)
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "Invalid project id"})
				return
			}
			var testID int
			row := dbConn2.QueryRow("SELECT id FROM tests WHERE id = ?", projectID)
			if err := row.Scan(&testID); err != nil {
				log.Printf("[ERROR] Project not found for test-status: %d", projectID)
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": "Project not found"})
				return
			}
			testRunRepo := repo.NewTestRunRepository(dbConn2)
			runs, _ := testRunRepo.GetTestRunsByTest(testID, 1, 0)
			if len(runs) == 0 {
				log.Printf("[ERROR] No test runs found for project: %d", projectID)
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": "No test runs found"})
				return
			}
			run := runs[0]
			log.Printf("[INFO] Test run status: project_id=%d, run_id=%d, status=%s", projectID, run.ID, run.Status)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"run_id":       run.ID,
				"status":       run.Status,
				"started_at":   run.StartedAt,
				"completed_at": run.CompletedAt,
			})
			return
		} else if r.Method == "PUT" || r.Method == "DELETE" {
			// Handle /projects/{id} (PUT, DELETE)
			prefix := "/projects/"
			idStr := path[len(prefix):]
			testID, err := strconv.Atoi(idStr)
			if err != nil {
				log.Printf("[ERROR] Invalid project id for %s: %v", r.Method, err)
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "Invalid project id"})
				return
			}
			if r.Method == "DELETE" {
				log.Printf("[INFO] Deleting project id=%d", testID)
				err := testRepo.DeleteTest(testID)
				if err != nil {
					log.Printf("[ERROR] Failed to delete project id=%d: %v", testID, err)
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				log.Printf("[INFO] Project deleted: id=%d", testID)
				w.WriteHeader(http.StatusNoContent)
				return
			} else if r.Method == "PUT" {
				var updates map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
					log.Printf("[ERROR] Failed to decode update for project id=%d: %v", testID, err)
					w.WriteHeader(http.StatusBadRequest)
					json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				err := testRepo.UpdateTest(testID, updates)
				if err != nil {
					log.Printf("[ERROR] Failed to update project id=%d: %v", testID, err)
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				log.Printf("[INFO] Project updated: id=%d", testID)
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"message": "Project updated successfully"})
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// --- Add: Run Single Scenario Endpoint ---
	http.HandleFunc("/scenarios/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")

		log.Printf("[SCENARIO-RUN][BACKEND] Incoming request: method=%s, path=%s, remote=%s", r.Method, r.URL.Path, r.RemoteAddr)

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		path := r.URL.Path
		if !strings.HasSuffix(path, "/run") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// Parse scenario ID
		prefix := "/scenarios/"
		suffix := "/run"
		idStr := path[len(prefix) : len(path)-len(suffix)]
		idStr = strings.TrimSuffix(idStr, "/")
		scenarioID, err := strconv.Atoi(idStr)
		if err != nil {
			log.Printf("[SCENARIO-RUN][ERROR] Invalid scenario ID: %s", idStr)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid scenario ID"})
			return
		}
		log.Printf("[SCENARIO-RUN] Running scenario id=%d", scenarioID)

		scenarioRepo := repo.NewScenarioRepository(dbConn2)
		scenario, err := scenarioRepo.GetScenarioByID(scenarioID)
		if err != nil || scenario == nil {
			log.Printf("[SCENARIO-RUN][ERROR] Scenario not found: %v", err)
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Scenario not found"})
			return
		}
		log.Printf("[SCENARIO-RUN] Fetched scenario: %+v", scenario)

		testRunRepo := repo.NewTestRunRepository(dbConn2)
		testID, err := strconv.Atoi(scenario.TestID)
		if err != nil {
			log.Printf("[SCENARIO-RUN][ERROR] Invalid test_id for scenario: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid test_id for scenario"})
			return
		}
		runID, err := testRunRepo.CreateTestRun(testID, nil)
		if err != nil {
			log.Printf("[SCENARIO-RUN][ERROR] Failed to create test run: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create test run"})
			return
		}
		log.Printf("[SCENARIO-RUN] Created test run: id=%d", runID)

		// Prepare agent
		llmClient, err := llm.NewLLMClient(llm.CohereProvider, llm.CohereModel)
		if err != nil {
			log.Printf("[SCENARIO-RUN][ERROR] Failed to create LLM client: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create LLM client"})
			return
		}
		initialState := llm.CurrentState{
			History:   []llm.HistoryItem{},
			TurnCount: 0,
			MaxTurns:  10,
			Fulfilled: false,
		}
		testName := ""
		row := dbConn2.QueryRow("SELECT name FROM tests WHERE id = ?", scenario.TestID)
		row.Scan(&testName)
		agent := agent.NewAgent(testName, scenario.Description, scenario.ExpectedOutput, initialState, llmClient, dbConn2)

		// Run agent
		state, err := agent.Run()
		if err != nil {
			log.Printf("[SCENARIO-RUN][ERROR] Agent run failed: %v", err)
			testRunRepo.UpdateTestRunStatus(runID, "failed")
			scenarioRepo.UpdateScenario(scenarioID, map[string]interface{}{"status": "Fail"})
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Agent run failed"})
			return
		}
		log.Printf("[SCENARIO-RUN] Agent run completed. Fulfilled: %v, Turns: %d", state.Fulfilled, state.TurnCount)

		// Record result in DB
		status := "Pass"
		if !state.Fulfilled {
			status = "Fail"
		}
		testRunRepo.UpdateTestRunStatus(runID, "completed")
		scenarioRepo.UpdateScenario(scenarioID, map[string]interface{}{"status": status})

		// Record interactions
		interactionRepo := repo.NewInteractionRepository(dbConn2)
		for _, h := range state.History {
			interaction := repo.Interaction{
				TestRunID:   runID,
				ScenarioID:  scenarioID,
				TurnNumber:  int(h.Turn),
				UserMessage: h.User,
				LLMResponse: h.Assistant,
			}
			interactionRepo.Create(&interaction)
		}

		log.Printf("[SCENARIO-RUN] Results recorded in DB for scenario id=%d, run_id=%d", scenarioID, runID)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"run_id":      runID,
			"scenario_id": scenarioID,
			"status":      status,
			"turns":       state.TurnCount,
			"fulfilled":   state.Fulfilled,
		})
	})

	log.Println("API server running on :8080 ... (GET, POST /projects)")
	http.ListenAndServe(":8080", nil)
}
