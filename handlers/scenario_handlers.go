package handlers

import (
	"encoding/json"
	repo "evaluator/repository" // Ensure this import path is correct
	"log"
	"net/http"
	"strconv"
	"strings"
)

// UploadScenariosHandler handles POST /api/upload-scenarios
func (env *APIEnv) UploadScenariosHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("[UPLOAD-SCENARIOS] Received request: method=%s, remote=%s, path=%s", r.Method, r.RemoteAddr, r.URL.Path)
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type ScenarioUploadPayload struct {
		TestID    string `json:"test_id"`
		Scenarios []struct {
			Description    string `json:"description"`
			ExpectedOutput string `json:"expected_output"`
		} `json:"scenarios"`
	}

	var payload ScenarioUploadPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("[UPLOAD-SCENARIOS] Error parsing payload from %s: %v", r.RemoteAddr, err)
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	testID, err := strconv.Atoi(payload.TestID)
	if err != nil {
		log.Printf("[UPLOAD-SCENARIOS] Invalid test_id value from %s: '%s', %v", r.RemoteAddr, payload.TestID, err)
		http.Error(w, "test_id must be an integer", http.StatusBadRequest)
		return
	}

	// Validate that the test_id exists
	_, err = env.TestRepo.GetTestByID(testID)
	if err != nil {
		log.Printf("[UPLOAD-SCENARIOS] Invalid test_id, project/test not found: %d, %v", testID, err)
		http.Error(w, "test_id does not refer to a valid test/project", http.StatusBadRequest) // Or 404 if preferred
		return
	}


	log.Printf("[UPLOAD-SCENARIOS] Received payload for test_id=%d from %s: %d scenarios", testID, r.RemoteAddr, len(payload.Scenarios))

	results := make([]map[string]any, 0)
	for _, s := range payload.Scenarios {
		log.Printf("[UPLOAD-SCENARIOS] Creating scenario for test_id=%d: description=\"%s\"", testID, s.Description)
		// Using env.ScenarioRepo now
		sc, err := env.ScenarioRepo.CreateScenario(testID, s.Description, s.ExpectedOutput)
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
				"scenario_id": sc.ID, // ID is a string as per Scenario struct
			})
		}
	}

	log.Printf("[UPLOAD-SCENARIOS] Sending response to %s: %d results processed", r.RemoteAddr, len(results))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated) // 201 since resources (scenarios) were created.
	json.NewEncoder(w).Encode(map[string]any{"results": results})
}

// GetScenariosByTestIDHandler handles GET /api/scenarios/{test_id}
func (env *APIEnv) GetScenariosByTestIDHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Path expected /api/scenarios/{test_id}
	// This handler will be registered at "/api/scenarios/"
	// r.URL.Path will be like "/api/scenarios/123"
	prefix := "/api/scenarios/"
	testIdStr := strings.TrimPrefix(r.URL.Path, prefix)
	testIdStr = strings.TrimSuffix(testIdStr, "/")


	if testIdStr == "" {
		log.Printf("[GET-SCENARIOS] Missing test_id in path: %s", r.URL.Path)
		http.Error(w, "Missing test_id in path", http.StatusBadRequest)
		return
	}

	testID, err := strconv.Atoi(testIdStr)
	if err != nil {
		log.Printf("[GET-SCENARIOS] Invalid test_id in path '%s': %v", testIdStr, err)
		http.Error(w, "Invalid test ID format", http.StatusBadRequest)
		return
	}

	log.Printf("[GET-SCENARIOS] Fetching scenarios for test_id=%d from %s", testID, r.RemoteAddr)
	// Using env.ScenarioRepo now
	scenarios, err := env.ScenarioRepo.GetScenariosByTestID(testID)
	if err != nil {
		// Check if error is sql.ErrNoRows, then return 404, otherwise 500
		// For now, generic 500, but could be improved.
		log.Printf("[GET-SCENARIOS] Error fetching scenarios for test_id=%d: %v", testID, err)
		http.Error(w, "Failed to retrieve scenarios: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if scenarios == nil { // Ensure scenarios is not nil if GetScenariosByTestID can return nil for no rows without error
		scenarios = []repo.Scenario{} // Return empty list instead of null
	}

	// Transforming to desired output format (original code did this)
	out := make([]map[string]any, 0, len(scenarios))
	for _, s := range scenarios {
		out = append(out, map[string]any{
			"id":              s.ID, // ID is string
			"description":     s.Description,
			"expected_output": s.ExpectedOutput,
			"status":          s.Status,
		})
	}

	log.Printf("[GET-SCENARIOS] Successfully fetched %d scenarios for test_id=%d", len(out), testID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
