package handlers

import (
	"encoding/json"
	repo "evaluator/repository"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// ListInteractionsByTestRunHandler handles GET /api/interactions/{testRunID}
func (env *APIEnv) ListInteractionsByTestRunHandler(w http.ResponseWriter, r *http.Request) {
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

	// Path expected: /api/interactions/{testRunID}
	prefix := "/api/interactions/"
	testRunIDStr := strings.TrimPrefix(r.URL.Path, prefix)
	testRunIDStr = strings.TrimSuffix(testRunIDStr, "/")

	if testRunIDStr == "" {
		log.Printf("[GET-INTERACTIONS] Missing testRunID in path: %s", r.URL.Path)
		http.Error(w, "Missing testRunID in path", http.StatusBadRequest)
		return
	}
	testRunID, err := strconv.Atoi(testRunIDStr)
	if err != nil {
		log.Printf("[GET-INTERACTIONS] Invalid testRunID in path '%s': %v", testRunIDStr, err)
		http.Error(w, "Invalid testRunID format", http.StatusBadRequest)
		return
	}

	interactions, err := env.InteractionRepo.GetByTestRunID(testRunID)
	if err != nil {
		log.Printf("[GET-INTERACTIONS] Error fetching interactions for testRunID=%d: %v", testRunID, err)
		http.Error(w, "Failed to retrieve interactions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if interactions == nil {
		interactions = []repo.Interaction{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(interactions)
}

// CreateInteractionHandler handles POST /api/interactions
func (env *APIEnv) CreateInteractionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var interaction repo.Interaction
	if err := json.NewDecoder(r.Body).Decode(&interaction); err != nil {
		log.Printf("[CREATE-INTERACTION] Failed to decode request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := env.InteractionRepo.Create(&interaction); err != nil {
		log.Printf("[CREATE-INTERACTION] Failed to create interaction: %v", err)
		http.Error(w, "Failed to create interaction", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(interaction)
}
