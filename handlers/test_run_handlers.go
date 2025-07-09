package handlers

import (
	"encoding/json"
	"evaluator/agent"
	"evaluator/llm"
	repo "evaluator/repository" // Ensure this import path is correct
	"log"
	"net/http"
	"strconv"
	"strings"
)

// handleRunProjectTest contains the logic for running all scenarios in a project.
// It's called by ProjectDispatchHandler.
func (env *APIEnv) handleRunProjectTest(w http.ResponseWriter, r *http.Request, projectID int) {
	// Note: CORS headers are expected to be set by the calling handler (ProjectDispatchHandler)
	// or a middleware. If called directly, ensure CORS is handled.
	log.Printf("[PROJ-RUN][HELPER] Test run initiated for project_id=%d", projectID)

	// Using env.TestRunRepo now
	runID, err := env.TestRunRepo.CreateTestRun(projectID, nil) // Assuming nil is acceptable for ScenariosJson initially
	if err != nil {
		log.Printf("[PROJ-RUN][BACKEND][ERROR] Failed to create test run entry for project_id=%d: %v", projectID, err)
		http.Error(w, "Failed to create test run entry", http.StatusInternalServerError)
		return
	}
	log.Printf("[PROJ-RUN][BACKEND] Test run entry created: project_id=%d, run_id=%d. Starting background process.", projectID, runID)

	go func(currentProjectID, currentRunID int) {
		log.Printf("[PROJ-RUN][GOROUTINE] Running test in background: project_id=%d, run_id=%d", currentProjectID, currentRunID)
		env.TestRunRepo.UpdateTestRunStatus(currentRunID, "running")

		// --- Full Agent Logic for all scenarios in a project ---
		scenarios, err := env.ScenarioRepo.GetScenariosByTestID(currentProjectID)
		if err != nil {
			log.Printf("[PROJ-RUN][GOROUTINE][ERROR] Failed to fetch scenarios for project_id=%d, run_id=%d: %v", currentProjectID, currentRunID, err)
			env.TestRunRepo.UpdateTestRunStatus(currentRunID, "failed")
			// Potentially update individual scenarios to "Error" or "Skipped"
			return
		}

		if len(scenarios) == 0 {
			log.Printf("[PROJ-RUN][GOROUTINE][WARN] No scenarios found for project_id=%d, run_id=%d. Marking as completed.", currentProjectID, currentRunID)
			env.TestRunRepo.UpdateTestRunStatus(currentRunID, "completed") // Or a different status like "no_scenarios"
			return
		}

		testProject, err := env.TestRepo.GetTestByID(currentProjectID)
		if err != nil {
			log.Printf("[PROJ-RUN][GOROUTINE][ERROR] Failed to fetch test/project details for project_id=%d: %v", currentProjectID, err)
			env.TestRunRepo.UpdateTestRunStatus(currentRunID, "failed")
			return
		}

		llmClient, err := llm.NewLLMClient(llm.CohereProvider, llm.CohereModel) // TODO: Make provider/model configurable
		if err != nil {
			log.Printf("[PROJ-RUN][GOROUTINE][ERROR] Failed to create LLM client for run_id=%d: %v", currentRunID, err)
			env.TestRunRepo.UpdateTestRunStatus(currentRunID, "failed")
			return
		}

		overallSuccess := true
		for _, sc := range scenarios {
			log.Printf("[PROJ-RUN][GOROUTINE] Starting agent for scenario_id=%s, run_id=%d", sc.ID, currentRunID)
			initialState := llm.CurrentState{
				History:   []llm.HistoryItem{},
				TurnCount: 0,
				MaxTurns:  int16(testProject.MaxInteractions), // Use MaxInteractions from the project
				Fulfilled: false,
			}

			scenarioIDInt, _ := strconv.Atoi(sc.ID) // Interaction repo expects int

			// Agent expects DB connection, pass env.DB
			testingAgent := agent.NewAgent(testProject.Name, sc.Description, sc.ExpectedOutput, initialState, llmClient, env.DB)

			finalState, finaljudgement, agentErr := testingAgent.Run()
			currentScenarioStatus := finaljudgement.Judgement

			if agentErr != nil {
				log.Printf("[PROJ-RUN][GOROUTINE][ERROR] Agent run failed for scenario_id=%s, run_id=%d: %v", sc.ID, currentRunID, agentErr)
				currentScenarioStatus = "Error"
			} else if !finalState.Fulfilled {
				log.Printf("[PROJ-RUN][GOROUTINE][INFO] Agent run completed but not fulfilled for scenario_id=%s, run_id=%d. Turns: %d", sc.ID, currentRunID, finalState.TurnCount)
				currentScenarioStatus = "Fail"
				overallSuccess = false
			} else {
				log.Printf("[PROJ-RUN][GOROUTINE][INFO] Agent run successful for scenario_id=%s, run_id=%d. Fulfilled: %v, Turns: %d", sc.ID, currentRunID, finalState.Fulfilled, finalState.TurnCount)
			}

			// Update individual scenario status
			env.ScenarioRepo.UpdateScenario(scenarioIDInt, map[string]interface{}{"status": currentScenarioStatus})

			// Record interactions for this scenario
			for _, h := range finalState.History {
				interaction := repo.Interaction{
					TestRunID:   currentRunID,
					ScenarioID:  scenarioIDInt,
					TurnNumber:  int(h.Turn),
					UserMessage: h.User,
					LLMResponse: h.Assistant,
				}
				err := env.InteractionRepo.Create(&interaction)
				if err != nil {
					log.Printf("[PROJ-RUN][GOROUTINE][ERROR] Failed to record interaction for scenario_id=%s, run_id=%d, turn=%d: %v", sc.ID, currentRunID, h.Turn, err)
				}
			}
		}

		finalStatus := "completed"
		if !overallSuccess {
			// finalStatus = "completed_with_failures" // Or keep "completed" and rely on scenario statuses
		}
		env.TestRunRepo.UpdateTestRunStatus(currentRunID, finalStatus)
		log.Printf("[PROJ-RUN][GOROUTINE] Test run completed: project_id=%d, run_id=%d. Overall success: %t", currentProjectID, currentRunID, overallSuccess)

	}(projectID, runID) // Pass current projectID and newly created runID

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted) // 202 Accepted as the test runs in background
	json.NewEncoder(w).Encode(map[string]interface{}{"run_id": runID, "status": "started"})
}

func (env *APIEnv) handleGetProjectTestStatus(w http.ResponseWriter, r *http.Request, projectID int) {
	log.Printf("[PROJ-RUN][INFO] Getting test status for project_id=%d", projectID)

	// Using env.TestRunRepo
	// Fetch the latest run for this project. The original code fetched only 1.
	runs, err := env.TestRunRepo.GetTestRunsByTest(projectID, 1, 0) // Limit 1, Offset 0 to get the latest
	if err != nil {
		log.Printf("[PROJ-RUN][ERROR] Failed to get test runs for project_id=%d: %v", projectID, err)
		http.Error(w, "Failed to retrieve test run status", http.StatusInternalServerError)
		return
	}

	if len(runs) == 0 {
		log.Printf("[PROJ-RUN][WARN] No test runs found for project_id=%d", projectID)
		http.Error(w, "No test runs found for this project", http.StatusNotFound)
		return
	}

	run := runs[0] // Get the latest one
	log.Printf("[PROJ-RUN][INFO] Latest test run status for project_id=%d: run_id=%d, status=%s", projectID, run.ID, run.Status)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"run_id":       run.ID,
		"status":       run.Status,
		"started_at":   run.StartedAt,
		"completed_at": run.CompletedAt,
		// Potentially add more details like success/failure counts from scenarios if available
	})
}

// ScenarioRunHandler manages POST /scenarios/{id}/run
// It's intended to be registered at a path like "/scenarios/"
func (env *APIEnv) ScenarioRunHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")

	log.Printf("[SCENARIO-RUN][BACKEND] ScenarioRunHandler hit: method=%s, path=%s, remote=%s", r.Method, r.URL.Path, r.RemoteAddr)

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		log.Printf("[SCENARIO-RUN][WARN] Method not allowed for scenario run: %s", r.Method)
		http.Error(w, "Method Not Allowed, expected POST", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path
	// Expected path format: /scenarios/{scenarioID}/run
	parts := strings.Split(strings.TrimPrefix(path, "/scenarios/"), "/")
	if len(parts) < 2 || parts[1] != "run" { // Needs {id} and "run"
		log.Printf("[SCENARIO-RUN][ERROR] Path too short or malformed for run: %s", path)
		http.Error(w, "Malformed request path for scenario run", http.StatusBadRequest)
		return
	}

	scenarioIDStr := parts[0]
	scenarioID, err := strconv.Atoi(scenarioIDStr)
	if err != nil {
		log.Printf("[SCENARIO-RUN][ERROR] Invalid scenario ID '%s' in path '%s': %v", scenarioIDStr, path, err)
		http.Error(w, "Invalid scenario ID format", http.StatusBadRequest)
		return
	}

	log.Printf("[SCENARIO-RUN] Initiating async run for scenario_id=%d", scenarioID)

	// Fetch scenario details
	scenario, err := env.ScenarioRepo.GetScenarioByID(scenarioID)
	if err != nil || scenario == nil {
		log.Printf("[SCENARIO-RUN][ERROR] Scenario not found: id=%d, err: %v", scenarioID, err)
		http.Error(w, "Scenario not found", http.StatusNotFound)
		return
	}
	log.Printf("[SCENARIO-RUN] Fetched scenario details: id=%s, test_id=%s", scenario.ID, scenario.TestID)

	// Fetch project/test details to get MaxInteractions and Name
	testID, err := strconv.Atoi(scenario.TestID)
	if err != nil {
		log.Printf("[SCENARIO-RUN][ERROR] Invalid test_id ('%s') associated with scenario_id=%d: %v", scenario.TestID, scenarioID, err)
		http.Error(w, "Invalid test_id for the scenario", http.StatusInternalServerError)
		return
	}
	testProject, err := env.TestRepo.GetTestByID(testID)
	if err != nil {
		log.Printf("[SCENARIO-RUN][ERROR] Could not fetch Test/Project details for test_id=%d: %v", testID, err)
		http.Error(w, "Failed to fetch project details for scenario run", http.StatusInternalServerError)
		return
	}

	// STEP 1: Immediately update status to "Running" in DB
	if _, err := env.ScenarioRepo.UpdateScenario(scenarioID, map[string]interface{}{"status": "Running"}); err != nil {
		log.Printf("[SCENARIO-RUN][ERROR] Failed to update scenario status to running for id=%d: %v", scenarioID, err)
		http.Error(w, "Failed to start run: could not update status", http.StatusInternalServerError)
		return
	}

	// STEP 2: Start the long-running process in a goroutine
	go func(sID int, proj *repo.Test, scen *repo.Scenario) {
		log.Printf("[SCENARIO-RUN][GOROUTINE] Starting agent for scenario_id=%d", sID)

		runID, err := env.TestRunRepo.CreateTestRun(proj.ID, nil)
		if err != nil {
			log.Printf("[SCENARIO-RUN][GOROUTINE][ERROR] Failed to create test run entry for scenario_id=%d: %v", sID, err)
			if _, err := env.ScenarioRepo.UpdateScenario(sID, map[string]interface{}{"status": "Fail"}); err != nil {
				log.Printf("[SCENARIO-RUN][GOROUTINE][ERROR] Failed to update scenario status to Fail for scenario_id=%d: %v", sID, err)
			}
			return
		}
		env.TestRunRepo.UpdateTestRunStatus(runID, "running")

		llmClient, err := llm.NewLLMClient(llm.CohereProvider, llm.CohereModel)
		if err != nil {
			log.Printf("[SCENARIO-RUN][GOROUTINE][ERROR] Failed to create LLM client for scenario_id=%d: %v", sID, err)
			env.TestRunRepo.UpdateTestRunStatus(runID, "failed")
			if _, err := env.ScenarioRepo.UpdateScenario(sID, map[string]interface{}{"status": "Fail"}); err != nil {
				log.Printf("[SCENARIO-RUN][GOROUTINE][ERROR] Failed to update scenario status to Fail for scenario_id=%d: %v", sID, err)
			}
			return
		}

		initialState := llm.CurrentState{
			History:   []llm.HistoryItem{},
			TurnCount: 0,
			MaxTurns:  int16(proj.MaxInteractions),
			Fulfilled: false,
		}
		testingAgent := agent.NewAgent(proj.Name, scen.Description, scen.ExpectedOutput, initialState, llmClient, env.DB)
		finalState, finalJudgement, agentErr := testingAgent.Run()

		runStatus := "completed"
		scenarioStatus := finalJudgement.Judgement
		if agentErr != nil || !finalState.Fulfilled {
			runStatus = "failed"
			if agentErr != nil {
				scenarioStatus = "Fail"
			}
		}

		env.TestRunRepo.UpdateTestRunStatus(runID, runStatus)
		if _, err := env.ScenarioRepo.UpdateScenario(sID, map[string]interface{}{"status": scenarioStatus}); err != nil {
			log.Printf("[SCENARIO-RUN][GOROUTINE][ERROR] Failed to update scenario status to %s for scenario_id=%d: %v", scenarioStatus, sID, err)
		}

		for _, h := range finalState.History {
			interaction := repo.Interaction{
				TestRunID:   runID,
				ScenarioID:  sID,
				TurnNumber:  int(h.Turn),
				UserMessage: h.User,
				LLMResponse: h.Assistant,
			}
			err := env.InteractionRepo.Create(&interaction)
			if err != nil {
				log.Printf("[SCENARIO-RUN][GOROUTINE][ERROR] Failed to record interaction for scenario_id=%d, run_id=%d, turn=%d: %v", sID, runID, h.Turn, err)
			}
		}

		log.Printf("[SCENARIO-RUN][GOROUTINE] Run finished for scenario_id=%d. Final status: %s", sID, scenarioStatus)
	}(scenarioID, testProject, scenario)

	// STEP 3: Immediately respond to the frontend
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":     "Scenario run initiated successfully",
		"scenario_id": scenarioID,
	})
}
