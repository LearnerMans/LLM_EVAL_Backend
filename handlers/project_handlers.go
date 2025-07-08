package handlers

import (
	"encoding/json"
	repo "evaluator/repository"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// ListCreateProjectsHandler handles GET /projects (list) and POST /projects (create).
func (env *APIEnv) ListCreateProjectsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.URL.Path != "/projects" {
		log.Printf("[PROJECTS][L/C][WARN] Path mismatch: expected /projects, got %s", r.URL.Path)
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case "GET":
		env.handleListProjects(w, r)
	case "POST":
		env.handleCreateProject(w, r)
	default:
		log.Printf("[PROJECTS][L/C][WARN] Method not allowed for /projects: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ProjectDispatchHandler is registered at "/projects/" and dispatches to specific handlers
// based on the path structure.
// - /projects/{id} (for PUT, DELETE) -> delegates to ProjectItemActionHandler logic
// - /projects/{id}/run-test -> delegates to ProjectTestRunHandler logic for run-test
// - /projects/{id}/test-status -> delegates to ProjectTestRunHandler logic for test-status
func (env *APIEnv) ProjectDispatchHandler(w http.ResponseWriter, r *http.Request) {
	// CORS headers are set by the specific sub-handlers if needed, or can be set here once.
	// For simplicity, let sub-handlers manage their specific CORS needs if they differ.
	// However, for OPTIONS, it's good to handle it broadly here.
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "PUT, DELETE, POST, GET, OPTIONS")


	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	path := r.URL.Path
	log.Printf("[PROJECTS][DISPATCH] Path: %s", path)

	// Expected path prefix: /projects/
	// parts[0] = {id}, parts[1] = action (optional)
	trimmedPath := strings.TrimPrefix(path, "/projects/")
	parts := strings.Split(trimmedPath, "/")

	if len(parts) == 0 || parts[0] == "" { // e.g. /projects/ or /projects//foo
		log.Printf("[PROJECTS][DISPATCH][ERROR] Project ID is missing or path malformed: %s", path)
		http.Error(w, "Project ID is missing or path malformed", http.StatusBadRequest)
		return
	}

	projectIDStr := parts[0]
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		log.Printf("[PROJECTS][DISPATCH][ERROR] Invalid project ID '%s' in path '%s': %v", projectIDStr, path, err)
		http.Error(w, "Invalid project ID format", http.StatusBadRequest)
		return
	}

	// Check if there's an action specified (e.g., "run-test", "test-status")
	if len(parts) > 1 {
		action := parts[1]
		// Delegate to ProjectTestRunHandler's logic directly
		// This avoids re-registering ProjectTestRunHandler and path parsing issues.
		// We pass the already parsed projectID and the action.
		switch action {
		case "run-test":
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed for run-test, expected POST", http.StatusMethodNotAllowed)
				return
			}
			env.handleRunProjectTest(w, r, projectID) // from test_run_handlers.go (needs to be accessible or logic moved)
			return
		case "test-status":
			if r.Method != http.MethodGet {
				http.Error(w, "Method not allowed for test-status, expected GET", http.StatusMethodNotAllowed)
				return
			}
			env.handleGetProjectTestStatus(w, r, projectID) // from test_run_handlers.go
			return
		default:
			log.Printf("[PROJECTS][DISPATCH][WARN] Unknown action '%s' for project %d", action, projectID)
			http.NotFound(w, r)
			return
		}
	} else {
		// No action, so it's for the project item itself (PUT, DELETE)
		// Delegate to ProjectItemActionHandler logic
		switch r.Method {
		case http.MethodPut:
			env.handleUpdateProject(w, r, projectID)
		case http.MethodDelete:
			env.handleDeleteProject(w, r, projectID)
		// Add GET /projects/{id} if desired
		// case http.MethodGet:
		// env.handleGetProject(w, r, projectID)
		default:
			log.Printf("[PROJECTS][DISPATCH][WARN] Method not allowed for /projects/%d: %s", projectID, r.Method)
			http.Error(w, "Method not allowed for this project resource", http.StatusMethodNotAllowed)
		}
		return
	}
}


// --- Helper methods (previously part of a combined handler or separate item handler) ---

func (env *APIEnv) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	log.Printf("[PROJECTS][HELPER][INFO] Creating new project (POST /projects) from %s", r.RemoteAddr)
	var newTest repo.Test
	if err := json.NewDecoder(r.Body).Decode(&newTest); err != nil {
		log.Printf("[PROJECTS][HELPER][ERROR] Failed to decode new project: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, err := env.TestRepo.CreateTest(newTest.Name, newTest.TenantID, newTest.ProjectID, newTest.MaxInteractions)
	if err != nil {
		log.Printf("[PROJECTS][HELPER][ERROR] Failed to create project: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	createdTest, err := env.TestRepo.GetTestByID(id)
	if err != nil {
		log.Printf("[PROJECTS][HELPER][ERROR] Failed to fetch created project: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[PROJECTS][HELPER][INFO] Project created: id=%d, name=%s", createdTest.ID, createdTest.Name)
	projectResponse := map[string]any{
		"id":               createdTest.ID,
		"title":            createdTest.Name,
		"tenant_id":        createdTest.TenantID,
		"project_id":       createdTest.ProjectID,
		"max_interactions": createdTest.MaxInteractions,
		"created_at":       createdTest.CreatedAt,
		"scenarios":        []repo.Scenario{},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(projectResponse)
}

func (env *APIEnv) handleListProjects(w http.ResponseWriter, r *http.Request) {
	log.Printf("[PROJECTS][HELPER][INFO] Listing all projects (GET /projects) from %s", r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")

	rows, err := env.DB.Query("SELECT id, name, tenant_id, project_id, max_interactions, created_at FROM tests")
	if err != nil {
		log.Printf("[PROJECTS][HELPER][ERROR] Failed to query projects: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var projectsResponse []map[string]any
	for rows.Next() {
		var t repo.Test
		if err := rows.Scan(&t.ID, &t.Name, &t.TenantID, &t.ProjectID, &t.MaxInteractions, &t.CreatedAt); err != nil {
			log.Printf("[PROJECTS][HELPER][ERROR] Failed to scan project row: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		scenarios, errScn := env.ScenarioRepo.GetScenariosByTestID(t.ID)
		if errScn != nil {
			log.Printf("[PROJECTS][HELPER][WARN] Failed to fetch scenarios for project id=%d: %v", t.ID, errScn)
		}

		projectItem := map[string]any{
			"id":               t.ID,
			"title":            t.Name,
			"tenant_id":        t.TenantID,
			"project_id":       t.ProjectID,
			"max_interactions": t.MaxInteractions,
			"created_at":       t.CreatedAt,
			"scenarios":        scenarios,
		}
		projectsResponse = append(projectsResponse, projectItem)
	}
	if rows.Err() != nil {
		log.Printf("[PROJECTS][HELPER][ERROR] Error iterating project rows: %v", rows.Err())
		http.Error(w, rows.Err().Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[PROJECTS][HELPER][INFO] Returned %d projects", len(projectsResponse))
	json.NewEncoder(w).Encode(projectsResponse)
}

func (env *APIEnv) handleUpdateProject(w http.ResponseWriter, r *http.Request, projectID int) {
	log.Printf("[PROJECTS][HELPER][INFO] Updating project id=%d (PUT /projects/%d) from %s", projectID, projectID, r.RemoteAddr)
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		log.Printf("[PROJECTS][HELPER][ERROR] Failed to decode update for project id=%d: %v", projectID, err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := env.TestRepo.UpdateTest(projectID, updates)
	if err != nil {
		log.Printf("[PROJECTS][HELPER][ERROR] Failed to update project id=%d: %v", projectID, err)
		http.Error(w, "Failed to update project", http.StatusInternalServerError)
		return
	}

	log.Printf("[PROJECTS][HELPER][INFO] Project updated: id=%d", projectID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Project updated successfully"})
}

func (env *APIEnv) handleDeleteProject(w http.ResponseWriter, r *http.Request, projectID int) {
	log.Printf("[PROJECTS][HELPER][INFO] Deleting project id=%d (DELETE /projects/%d) from %s", projectID, projectID, r.RemoteAddr)
	err := env.TestRepo.DeleteTest(projectID)
	if err != nil {
		log.Printf("[PROJECTS][HELPER][ERROR] Failed to delete project id=%d: %v", projectID, err)
		http.Error(w, "Failed to delete project", http.StatusInternalServerError)
		return
	}
	log.Printf("[PROJECTS][HELPER][INFO] Project deleted: id=%d", projectID)
	w.WriteHeader(http.StatusNoContent)
}

// Note: handleRunProjectTest and handleGetProjectTestStatus are defined in test_run_handlers.go.
// To make them callable from ProjectDispatchHandler, they either need to be public methods
// of APIEnv (which they are) or their logic duplicated/refactored.
// Current design assumes they are public methods on APIEnv and can be called.
// This means `project_handlers.go` and `test_run_handlers.go` must agree on these method signatures if called across.
// Alternatively, `ProjectDispatchHandler` could be part of `test_run_handlers.go` if it mostly deals with runs.
// For now, keeping dispatch logic in `project_handlers.go` for `/projects/` path.
// The methods `env.handleRunProjectTest` and `env.handleGetProjectTestStatus` are indeed part of `test_run_handlers.go` but are not exported from the `handlers` package as they are not handlers themselves but helper methods for a handler.
// The solution is to make the actual HTTP handlers in `test_run_handlers.go` (like `ProjectTestRunHandler`) also parse the projectID and action,
// and then `ProjectDispatchHandler` would call these *exported handler methods*.
// Or, more simply, the logic of `handleRunProjectTest` and `handleGetProjectTestStatus` must be available to `ProjectDispatchHandler`.
// The simplest way is to make them public methods of APIEnv if they are not already, or move their core logic to public methods.

// The current `test_run_handlers.go` has `ProjectTestRunHandler` which does parsing.
// We need `ProjectDispatchHandler` to call the *logic* not the *handler*.
// The helper methods `handleRunProjectTest` and `handleGetProjectTestStatus` are already methods on `*APIEnv`
// and are defined in `test_run_handlers.go`. They are not exported from the package `handlers` but are callable
// by any method of `*APIEnv` within the same package. This is fine.
// My `overwrite_file_with_block` for `test_run_handlers.go` made them unexported helpers for `ProjectTestRunHandler` and `ScenarioRunHandler`.
// Let me ensure `handleRunProjectTest` and `handleGetProjectTestStatus` are methods on `APIEnv` and thus accessible within the `handlers` package.
// Yes, they are defined as `func (env *APIEnv) handleRunProjectTest(...)` etc., in `test_run_handlers.go`.
// So, they are accessible to `ProjectDispatchHandler` in `project_handlers.go` as they are in the same package.
