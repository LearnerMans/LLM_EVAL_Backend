package repository

import (
	"database/sql"
)

type TestRun struct {
	ID               int
	ScenarioID       int
	Status           string
	Verdict          string
	VerdictReasoning string
	StartedAt        string
	CompletedAt      *string
}

type TestRunRepo interface {
	CreateTestRun(scenarioID int, metadata map[string]interface{}) (int, error)
	GetTestRunByID(testRunID int) (*TestRun, error)
	UpdateTestRunStatus(testRunID int, status string, verdict *string, verdictReasoning *string) error
	GetTestRunsByScenario(scenarioID int, limit, offset int) ([]TestRun, error)
	GetRecentTestRuns(limit int, tenantID, projectID *string) ([]TestRun, error)
	GetTestRunStats(scenarioID int, filter map[string]interface{}) (map[string]interface{}, error)
	GetFailedTestRuns(filter map[string]interface{}) ([]TestRun, error)
	GetTestRunsInRange(startDate, endDate string, scenarioID, tenantID *int) ([]TestRun, error)
	DeleteOldTestRuns(cutoffDate string) error
	ArchiveCompletedRuns(criteria map[string]interface{}) error
	GetTestExecutionSummary(filter map[string]interface{}) (map[string]interface{}, error)
	GetTestRunsByTest(testID int, limit, offset int) ([]TestRun, error)
}

type TestRunRepository struct {
	db *sql.DB
}

func NewTestRunRepository(db *sql.DB) TestRunRepo {
	return &TestRunRepository{db: db}
}

func (r *TestRunRepository) ArchiveCompletedRuns(criteria map[string]interface{}) error {
	// TODO: Implement archiving logic
	return nil
}

func (r *TestRunRepository) CreateTestRun(scenarioID int, metadata map[string]interface{}) (int, error) {
	status := "Not Run"
	if val, ok := metadata["status"].(string); ok {
		status = val
	}
	stmt := `INSERT INTO runs (scenario_id, status) VALUES (?, ?)`
	res, err := r.db.Exec(stmt, scenarioID, status)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (r *TestRunRepository) DeleteOldTestRuns(cutoffDate string) error {
	// TODO: Implement deletion logic
	return nil
}

func (r *TestRunRepository) GetFailedTestRuns(filter map[string]interface{}) ([]TestRun, error) {
	// TODO: Implement failed test runs retrieval
	return nil, nil
}

func (r *TestRunRepository) GetRecentTestRuns(limit int, tenantID, projectID *string) ([]TestRun, error) {
	// TODO: Implement recent test runs retrieval
	return nil, nil
}

func (r *TestRunRepository) GetTestExecutionSummary(filter map[string]interface{}) (map[string]interface{}, error) {
	// TODO: Implement test execution summary
	return nil, nil
}

func (r *TestRunRepository) GetTestRunByID(testRunID int) (*TestRun, error) {
	stmt := `SELECT id, scenario_id, status, started_at, completed_at FROM runs WHERE id = ?`
	row := r.db.QueryRow(stmt, testRunID)
	var tr TestRun
	var completedAt sql.NullString
	if err := row.Scan(&tr.ID, &tr.ScenarioID, &tr.Status, &tr.StartedAt, &completedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if completedAt.Valid {
		tr.CompletedAt = &completedAt.String
	} else {
		tr.CompletedAt = nil
	}
	return &tr, nil
}

func (r *TestRunRepository) GetTestRunStats(scenarioID int, filter map[string]interface{}) (map[string]interface{}, error) {
	// TODO: Implement test run stats
	return nil, nil
}

func (r *TestRunRepository) GetTestRunsByScenario(scenarioID int, limit, offset int) ([]TestRun, error) {
	stmt := `SELECT id, scenario_id, status, started_at, completed_at, verdict, verdict_reasoning FROM runs WHERE scenario_id = ? ORDER BY started_at DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Query(stmt, scenarioID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []TestRun
	for rows.Next() {
		var tr TestRun
		var completedAt sql.NullString
		if err := rows.Scan(&tr.ID, &tr.ScenarioID, &tr.Status, &tr.StartedAt, &completedAt, &tr.Verdict, &tr.VerdictReasoning); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			tr.CompletedAt = &completedAt.String
		} else {
			tr.CompletedAt = nil
		}
		runs = append(runs, tr)
	}
	return runs, nil
}

func (r *TestRunRepository) GetTestRunsInRange(startDate, endDate string, scenarioID, tenantID *int) ([]TestRun, error) {
	// TODO: Implement get test runs in range
	return nil, nil
}

func (r *TestRunRepository) GetTestRunsByTest(testID int, limit, offset int) ([]TestRun, error) {
	stmt := `SELECT runs.id, runs.scenario_id, runs.status, runs.started_at, runs.completed_at
		FROM runs
		JOIN scenarios ON runs.scenario_id = scenarios.id
		WHERE scenarios.test_id = ?
		ORDER BY runs.started_at DESC
		LIMIT ? OFFSET ?`
	rows, err := r.db.Query(stmt, testID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []TestRun
	for rows.Next() {
		var tr TestRun
		var completedAt sql.NullString
		if err := rows.Scan(&tr.ID, &tr.ScenarioID, &tr.Status, &tr.StartedAt, &completedAt); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			tr.CompletedAt = &completedAt.String
		} else {
			tr.CompletedAt = nil
		}
		runs = append(runs, tr)
	}
	return runs, nil
}

func (r *TestRunRepository) UpdateTestRunStatus(testRunID int, status string, verdict *string, verdictReasoning *string) error {
	stmt := `UPDATE runs SET status = ?, verdict = COALESCE(?, verdict), verdict_reasoning = COALESCE(?, verdict_reasoning) WHERE id = ?`
	_, err := r.db.Exec(stmt, status, verdict, verdictReasoning, testRunID)
	return err
}
