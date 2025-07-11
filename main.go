package main

import (
	"evaluator/db"
	"evaluator/handlers"
	"log"
	"net/http"
	"strings"
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

	apiEnv := handlers.NewAPIEnv(dbConn)

	// --- Project/Test related routes ---
	http.HandleFunc("/projects", apiEnv.ListCreateProjectsHandler) // Manages "Tests"
	http.HandleFunc("/projects/", apiEnv.ProjectDispatchHandler)   // Manages "Tests" actions like /run-test, /test-status

	// --- Scenario related routes ---

	// Bulk upload scenarios (remains unchanged)
	http.HandleFunc("/api/upload-scenarios", apiEnv.UploadScenariosHandler)

	// Routes for scenarios related to a specific test:
	// GET /api/tests/{test_id}/scenarios - List scenarios for a test
	// POST /api/tests/{test_id}/scenarios - Create a new scenario for a test
	http.HandleFunc("/api/tests/", func(w http.ResponseWriter, r *http.Request) {
		// Path: /api/tests/{test_id}/scenarios
		trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/tests/")
		parts := strings.Split(trimmedPath, "/")

		// parts[0] should be {test_id}
		// parts[1] should be "scenarios"
		if len(parts) == 2 && parts[1] == "scenarios" {
			// testIDStr := parts[0] // The handler will parse this
			if r.Method == "GET" {
				apiEnv.GetScenariosByTestIDHandler(w, r) // Handler needs to parse test_id from this path
			} else if r.Method == "POST" {
				apiEnv.CreateScenarioHandler(w, r) // Handler needs to parse test_id from this path
			} else if r.Method == "OPTIONS" {
				// Delegate OPTIONS handling to one of the handlers, e.g., GET handler, as it sets common CORS headers
				// Or ensure both GET and POST handlers correctly respond to OPTIONS.
				// The individual handlers (CreateScenarioHandler, GetScenariosByTestIDHandler) already handle OPTIONS.
				// So, calling the appropriate one based on a typical subsequent request method is fine.
				// To be absolutely correct for CORS preflight, it should respond based on allowed methods for this path.
				// For simplicity here, we can let one of them handle it or add a generic OPTIONS response.
				// Let's try to call the relevant handler to ensure CORS headers are set as they expect.
				// This is a bit of a simplification for preflight; a dedicated OPTIONS handler per path pattern is more robust.
				// However, since our handlers are setting broad CORS headers, this should work.
				apiEnv.GetScenariosByTestIDHandler(w, r) // Or CreateScenarioHandler, doesn't matter much if CORS is permissive
			} else {
				http.Error(w, "Method not allowed for /api/tests/{test_id}/scenarios", http.StatusMethodNotAllowed)
			}
		} else {
			http.NotFound(w, r)
		}
	})

	// Routes for individual scenarios by scenario_id:
	// GET /api/scenarios/{scenario_id} - Get a specific scenario
	// PUT /api/scenarios/{scenario_id} - Update a specific scenario
	// DELETE /api/scenarios/{scenario_id} - Delete a specific scenario
	http.HandleFunc("/api/scenarios/", func(w http.ResponseWriter, r *http.Request) {
		// Path: /api/scenarios/{scenario_id}
		// Ensure path does not end with /run, /stop, /runs to avoid conflict with older routes

		trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/scenarios/")
		// Check if the trimmedPath contains another "/" which means it's not just an ID
		// e.g. /api/scenarios/123/something_else - this should be a NotFound
		// but /api/scenarios/123 should be handled.
		if strings.Contains(trimmedPath, "/") {
			// This check ensures we only handle /api/scenarios/{id} and not, for example,
			// a misrouted /api/scenarios/anything/else.
			// It also means that the old /api/scenarios/{test_id} (if it was still active) wouldn't be caught here.
			http.NotFound(w, r)
			return
		}
		// At this point, path is /api/scenarios/{scenario_id}
		// scenarioIDStr := trimmedPath // Handler will parse this

		switch r.Method {
		case "GET":
			apiEnv.GetScenarioHandler(w, r)
		case "PUT":
			apiEnv.UpdateScenarioHandler(w, r)
		case "DELETE":
			apiEnv.DeleteScenarioHandler(w, r)
		case "OPTIONS": // Handle OPTIONS for CORS preflight for all these methods under this path
			// Delegate to one of the handlers to set CORS headers.
			apiEnv.GetScenarioHandler(w,r)
		default:
			http.Error(w, "Method not allowed for /api/scenarios/{id}", http.StatusMethodNotAllowed)
		}
	})

	// Existing generic /scenarios/ handler for /run, /stop, /runs
	// The paths are distinct enough: "/api/scenarios/" vs "/scenarios/".
	// The Go standard mux chooses the handler that matches the longest prefix.
	// So, "/api/scenarios/123" will be handled by the `/api/scenarios/` handler.
	// And "/scenarios/123/run" will be handled by the `/scenarios/` handler.
	http.HandleFunc("/scenarios/", func(w http.ResponseWriter, r *http.Request) {
		// This will handle /scenarios/{id}/run, /scenarios/{id}/stop, /scenarios/{id}/runs

		// Defensive check: Ensure this is not an /api/ path
		if strings.HasPrefix(r.URL.Path, "/api/") {
			// This should ideally not be reached if /api/ routes are correctly handled by more specific prefixes.
			http.NotFound(w, r)
			return
		}

		if strings.HasSuffix(r.URL.Path, "/run") {
			apiEnv.ScenarioRunHandler(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/stop") {
			apiEnv.StopScenarioHandler(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/runs") && r.Method == "GET" {
			apiEnv.GetTestRunsByScenarioHandler(w, r)
		} else {
			// Fallback for other /scenarios/ endpoints not matching specific actions
			http.NotFound(w, r)
		}
	})

	// --- Interaction routes ---
	http.HandleFunc("/api/interactions/", apiEnv.ListInteractionsByTestRunHandler) // GET specific to test run
	http.HandleFunc("/api/interactions", apiEnv.CreateInteractionHandler)     // POST new interaction

	// --- Logging for registered routes (for verification) ---
	log.Println("Registered route: GET, POST /projects")
	log.Println("Registered route: (various) /projects/*")
	log.Println("Registered route: POST /api/upload-scenarios")

	log.Println("Registered route: GET, POST /api/tests/{test_id}/scenarios")
	log.Println("Registered route: GET, PUT, DELETE /api/scenarios/{scenario_id}")

	log.Println("Registered route: POST /scenarios/{id}/run")
	log.Println("Registered route: POST /scenarios/{id}/stop")
	log.Println("Registered route: GET /scenarios/{id}/runs")

	log.Println("Registered route: GET /api/interactions/{testRunID}")
	log.Println("Registered route: POST /api/interactions")


	log.Println("API server running on :8080 ...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
