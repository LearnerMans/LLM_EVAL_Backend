package handlers

import (
	"database/sql" // Required for sql.ErrNoRows
	"encoding/json"
	repo "evaluator/repository" // Ensure this import path is correct
	"fmt"                        // For fmt.Errorf
	"log"
	"net/http"
	"strconv"
	"strings"
)

// Helper function to write JSON response
func writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173") // Consider making this configurable
	w.WriteHeader(statusCode)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("Error encoding JSON response: %v", err)
			// Cannot write header again here, so just log
		}
	}
}

// Helper function to write error response
func writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	log.Printf("Error response: status=%d, message=%s", statusCode, message)
	writeJSONResponse(w, statusCode, map[string]string{"error": message})
}

// --- New CRUD Handlers ---

// CreateScenarioHandler handles POST /api/tests/{test_id}/scenarios
func (env *APIEnv) CreateScenarioHandler(w http.ResponseWriter, r *http.Request) {
	// CORS preflight
	if r.Method == "OPTIONS" {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")


	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/tests/"), "/")
	if len(parts) < 2 || parts[1] != "scenarios" {
		writeErrorResponse(w, http.StatusBadRequest, "Malformed URL: expected /api/tests/{test_id}/scenarios")
		return
	}
	testIDStr := parts[0]
	testID, err := strconv.Atoi(testIDStr)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid test_id in path: must be an integer.")
		return
	}

	var reqBody struct {
		Description    string `json:"description"`
		ExpectedOutput string `json:"expected_output"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	if reqBody.Description == "" || reqBody.ExpectedOutput == "" {
		writeErrorResponse(w, http.StatusBadRequest, "Description and expected_output are required.")
		return
	}

	// The repository's CreateScenario method already checks if test_id exists
	createdScenario, err := env.ScenarioRepo.CreateScenario(testID, reqBody.Description, reqBody.ExpectedOutput)
	if err != nil {
		// Check if the error is due to test_id not found (this depends on the error returned by repo)
		// For now, assuming a generic error, but can be more specific.
		// Example: if strings.Contains(err.Error(), "does not exist") { ... }
		log.Printf("Error creating scenario for test_id %d: %v", testID, err)
		// The repo layer now returns a more specific error for non-existent test
		if strings.Contains(err.Error(),"test with id") && strings.Contains(err.Error(),"does not exist"){
			writeErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		if strings.Contains(err.Error(),"invalid scenario format"){
			writeErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to create scenario: "+err.Error())
		return
	}

	log.Printf("Successfully created scenario id=%d for test_id=%d", createdScenario.ID, testID)
	writeJSONResponse(w, http.StatusCreated, createdScenario)
}

// GetScenarioHandler handles GET /api/scenarios/{scenario_id}
func (env *APIEnv) GetScenarioHandler(w http.ResponseWriter, r *http.Request) {
	// CORS preflight
	if r.Method == "OPTIONS" {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")


	scenarioIDStr := strings.TrimPrefix(r.URL.Path, "/api/scenarios/")
	scenarioID, err := strconv.Atoi(scenarioIDStr)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid scenario_id in path: must be an integer.")
		return
	}

	scenario, err := env.ScenarioRepo.GetScenarioByID(scenarioID)
	if err != nil {
		// The repository GetScenarioByID returns (nil, nil) for sql.ErrNoRows effectively
		// but the updated one returns (nil, specific error) or (nil, nil) if no rows
		// Let's assume repo returns sql.ErrNoRows which our current repo wrapper might hide.
		// The current repo GetScenarioByID returns (nil, nil) if not found.
		log.Printf("Error fetching scenario id %d: %v", scenarioID, err) // This err might be nil if not found by current repo design
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve scenario: "+err.Error())
		return
	}

	if scenario == nil { // This is the condition for "not found" based on current repo GetScenarioByID
		writeErrorResponse(w, http.StatusNotFound, fmt.Sprintf("Scenario with id %d not found.", scenarioID))
		return
	}

	log.Printf("Successfully fetched scenario id=%d", scenario.ID)
	writeJSONResponse(w, http.StatusOK, scenario)
}

// UpdateScenarioHandler handles PUT /api/scenarios/{scenario_id}
func (env *APIEnv) UpdateScenarioHandler(w http.ResponseWriter, r *http.Request) {
	// CORS preflight
	if r.Method == "OPTIONS" {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "PUT, OPTIONS")
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")


	scenarioIDStr := strings.TrimPrefix(r.URL.Path, "/api/scenarios/")
	scenarioID, err := strconv.Atoi(scenarioIDStr)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid scenario_id in path: must be an integer.")
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Ensure no attempt to update ID or TestID
	delete(updates, "id")
	delete(updates, "test_id")


	updatedScenario, err := env.ScenarioRepo.UpdateScenario(scenarioID, updates)
	if err != nil {
		log.Printf("Error updating scenario id %d: %v", scenarioID, err)
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(w, http.StatusNotFound, err.Error())
		} else if strings.Contains(err.Error(), "invalid scenario format") {
			writeErrorResponse(w, http.StatusBadRequest, err.Error())
		} else {
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to update scenario: "+err.Error())
		}
		return
	}

	log.Printf("Successfully updated scenario id=%d", updatedScenario.ID)
	writeJSONResponse(w, http.StatusOK, updatedScenario)
}

// DeleteScenarioHandler handles DELETE /api/scenarios/{scenario_id}
func (env *APIEnv) DeleteScenarioHandler(w http.ResponseWriter, r *http.Request) {
	// CORS preflight
	if r.Method == "OPTIONS" {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "DELETE, OPTIONS")
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")


	scenarioIDStr := strings.TrimPrefix(r.URL.Path, "/api/scenarios/")
	scenarioID, err := strconv.Atoi(scenarioIDStr)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid scenario_id in path: must be an integer.")
		return
	}

	err = env.ScenarioRepo.DeleteScenario(scenarioID)
	if err != nil {
		log.Printf("Error deleting scenario id %d: %v", scenarioID, err)
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(w, http.StatusNotFound, err.Error())
		} else {
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to delete scenario: "+err.Error())
		}
		return
	}

	log.Printf("Successfully deleted scenario id=%d", scenarioID)
	writeJSONResponse(w, http.StatusNoContent, nil)
}


// --- Existing Handlers (Modified for path change if necessary) ---

// UploadScenariosHandler handles POST /api/upload-scenarios
// This handler remains as is, its path is distinct and for bulk uploads.
// Its internal logic using CreateScenario will benefit from repository updates.
// Small correction: `sc.ID` is now int, ensure JSON response handles it.
func (env *APIEnv) UploadScenariosHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("[UPLOAD-SCENARIOS] Received request: method=%s, remote=%s, path=%s", r.Method, r.RemoteAddr, r.URL.Path)
	// CORS preflight
	if r.Method == "OPTIONS" {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")


	if r.Method != "POST" {
		log.Printf("[UPLOAD-SCENARIOS] Method not allowed: %s from %s", r.Method, r.RemoteAddr)
		writeErrorResponse(w,http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	type ScenarioUploadPayload struct {
		// TestID is string here due to original design, converted to int
		TestID    string `json:"test_id"`
		Scenarios []struct {
			Description    string `json:"description"`
			ExpectedOutput string `json:"expected_output"`
		} `json:"scenarios"`
	}

	var payload ScenarioUploadPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("[UPLOAD-SCENARIOS] Error parsing payload from %s: %v", r.RemoteAddr, err)
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	testID, err := strconv.Atoi(payload.TestID)
	if err != nil {
		log.Printf("[UPLOAD-SCENARIOS] Invalid test_id value from %s: '%s', %v", r.RemoteAddr, payload.TestID, err)
		writeErrorResponse(w, http.StatusBadRequest, "test_id must be an integer")
		return
	}

	// Validate that the test_id exists
	// Assuming TestRepo.GetTestByID returns (Test, error), and error is sql.ErrNoRows if not found
	_, err = env.TestRepo.GetTestByID(testID)
	if err != nil {
		if err == sql.ErrNoRows { // Make sure GetTestByID actually returns this for not found
			log.Printf("[UPLOAD-SCENARIOS] Invalid test_id, project/test not found: %d", testID)
			writeErrorResponse(w, http.StatusNotFound, fmt.Sprintf("test_id %d does not refer to a valid test/project", testID))
		} else {
			log.Printf("[UPLOAD-SCENARIOS] Error validating test_id %d: %v", testID, err)
			writeErrorResponse(w, http.StatusInternalServerError, "Error validating test_id: "+err.Error())
		}
		return
	}

	log.Printf("[UPLOAD-SCENARIOS] Received payload for test_id=%d from %s: %d scenarios", testID, r.RemoteAddr, len(payload.Scenarios))

	results := make([]map[string]any, 0)
	for _, s := range payload.Scenarios {
		log.Printf("[UPLOAD-SCENARIOS] Creating scenario for test_id=%d: description=\"%s\"", testID, s.Description)
		sc, errCreate := env.ScenarioRepo.CreateScenario(testID, s.Description, s.ExpectedOutput)
		if errCreate != nil {
			log.Printf("[UPLOAD-SCENARIOS] Error creating scenario for description=\"%s\": %v", s.Description, errCreate)
			results = append(results, map[string]any{
				"description": s.Description,
				"success":     false,
				"error":       errCreate.Error(),
			})
		} else {
			// sc.ID is now int
			log.Printf("[UPLOAD-SCENARIOS] Successfully created scenario id=%d for description=\"%s\"", sc.ID, s.Description)
			results = append(results, map[string]any{
				"description": s.Description,
				"success":     true,
				"scenario_id": sc.ID,
			})
		}
	}

	log.Printf("[UPLOAD-SCENARIOS] Sending response to %s: %d results processed", r.RemoteAddr, len(results))
	writeJSONResponse(w, http.StatusCreated, map[string]any{"results": results})
}

// GetScenariosByTestIDHandler handles GET /api/tests/{test_id}/scenarios (NEW PROPOSED PATH)
func (env *APIEnv) GetScenariosByTestIDHandler(w http.ResponseWriter, r *http.Request) {
	// CORS preflight
	if r.Method == "OPTIONS" {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")


	if r.Method != "GET" {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Path expected /api/tests/{test_id}/scenarios
	// Registered at "/api/tests/" and then further routed in main.go or a sub-router
	// For this handler, assume path is like "/api/tests/123/scenarios"
	// Example: if main.go registers this for path prefix "/api/tests/"
	// then r.URL.Path might be "/api/tests/123/scenarios"
	// We need to extract "123"

	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	// Expected: "api", "tests", "{test_id}", "scenarios"
	if len(pathParts) != 4 || pathParts[0] != "api" || pathParts[1] != "tests" || pathParts[3] != "scenarios" {
		writeErrorResponse(w, http.StatusBadRequest, "Malformed URL. Expected /api/tests/{test_id}/scenarios")
		return
	}
	testIDStr := pathParts[2]


	testID, err := strconv.Atoi(testIDStr)
	if err != nil {
		log.Printf("[GET-SCENARIOS-BY-TEST] Invalid test_id in path '%s': %v", testIDStr, err)
		writeErrorResponse(w, http.StatusBadRequest, "Invalid test ID format in path.")
		return
	}

	// Optional: Validate that the test_id exists
	_, err = env.TestRepo.GetTestByID(testID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[GET-SCENARIOS-BY-TEST] Test with id %d not found.", testID)
			writeErrorResponse(w, http.StatusNotFound, fmt.Sprintf("Test with id %d not found.", testID))
		} else {
			log.Printf("[GET-SCENARIOS-BY-TEST] Error validating test_id %d: %v", testID, err)
			writeErrorResponse(w, http.StatusInternalServerError, "Error validating test_id: "+err.Error())
		}
		return
	}


	log.Printf("[GET-SCENARIOS-BY-TEST] Fetching scenarios for test_id=%d from %s", testID, r.RemoteAddr)
	scenarios, err := env.ScenarioRepo.GetScenariosByTestID(testID)
	if err != nil {
		log.Printf("[GET-SCENARIOS-BY-TEST] Error fetching scenarios for test_id=%d: %v", testID, err)
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve scenarios: "+err.Error())
		return
	}

	if scenarios == nil { // Should not happen if repo returns empty slice for no rows
		scenarios = []repo.Scenario{}
	}

	// Scenarios are already in the correct struct with json tags due to repo changes.
	log.Printf("[GET-SCENARIOS-BY-TEST] Successfully fetched %d scenarios for test_id=%d", len(scenarios), testID)
	writeJSONResponse(w, http.StatusOK, scenarios)
}

// StopScenarioHandler handles POST /scenarios/{id}/stop
// This handler remains largely the same.
// Small correction: `scenarioID` is int. Response `scenario_id` should be int.
func (env *APIEnv) StopScenarioHandler(w http.ResponseWriter, r *http.Request) {
	// CORS preflight
	if r.Method == "OPTIONS" {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")


	if r.Method != "POST" {
		writeErrorResponse(w,http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/scenarios/"), "/")
	if len(parts) < 2 || parts[1] != "stop" {
		writeErrorResponse(w,http.StatusBadRequest, "Malformed request path for scenario stop. Expected /scenarios/{id}/stop")
		return
	}
	scenarioIDStr := parts[0]
	scenarioID, err := strconv.Atoi(scenarioIDStr)
	if err != nil {
		writeErrorResponse(w,http.StatusBadRequest, "Invalid scenario ID format")
		return
	}

	updatedScenario, err := env.ScenarioRepo.UpdateScenario(scenarioID, map[string]interface{}{"status": "Error"})
	if err != nil {
		log.Printf("[SCENARIO-STOP][ERROR] Failed to update scenario status to Error for id=%d: %v", scenarioID, err)
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(w, http.StatusNotFound, err.Error())
		} else {
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to update scenario status: "+err.Error())
		}
		return
	}

	log.Printf("Successfully stopped scenario id=%d, status set to %s", updatedScenario.ID, updatedScenario.Status)
	writeJSONResponse(w, http.StatusOK, updatedScenario) // Return the updated scenario
}
