package repository

import (
	"database/sql"
)

// Placeholder interfaces for TestRepo and TestRunRepo
// TODO: Implement these interfaces in their respective files

type Store struct {
	Interaction InteractionRepo
	Scenario    ScenarioRepo
	Test        TestRepo
	TestRun     TestRunRepo
}

func NewStore(db *sql.DB) *Store {
	return &Store{
		Interaction: NewInteractionRepository(db),
		Scenario:    NewScenarioRepository(db),
		Test:        NewTestRepository(db),
		TestRun:     NewTestRunRepository(db),
	}
}

func (s *Store) GetTestRunsByTest(testID int, limit, offset int) ([]TestRun, error) {
	return s.TestRun.GetTestRunsByTest(testID, limit, offset)
}
