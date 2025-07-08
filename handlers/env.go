package handlers

import (
	"database/sql"
	"evaluator/db" // Assuming db package is accessible
	repo "evaluator/repository"
)

// APIEnv holds application-wide dependencies for handlers.
type APIEnv struct {
	DB                *sql.DB
	TestRepo          repo.TestRepository
	ScenarioRepo      repo.ScenarioRepository
	TestRunRepo       repo.TestRunRepository
	InteractionRepo   repo.InteractionRepository
	// Add other dependencies like loggers, LLM clients if they need to be accessed by handlers
}

// NewAPIEnv creates a new APIEnv with all necessary dependencies.
// This function can be expanded to initialize more dependencies.
func NewAPIEnv(dbConn *sql.DB) *APIEnv {
	return &APIEnv{
		DB:                dbConn,
		TestRepo:          repo.NewTestRepository(dbConn),
		ScenarioRepo:      repo.NewScenarioRepository(dbConn),
		TestRunRepo:       repo.NewTestRunRepository(dbConn),
		InteractionRepo:   repo.NewInteractionRepository(dbConn),
	}
}
