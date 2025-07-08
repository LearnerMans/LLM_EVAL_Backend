package repository

import (
	"database/sql"
	"fmt"
	"strconv"
)

type Scenario struct {
	ID             string
	TestID         string
	Description    string
	ExpectedOutput string
	Status         string
}

type ScenarioRepo interface {
	// Phase 1
	CreateScenario(testID int, description, expectedOutput string) (*Scenario, error)
	GetScenariosByTestID(testID int) ([]Scenario, error)
	GetScenarioByID(scenarioID int) (*Scenario, error)
	UpdateScenario(scenarioID int, updates map[string]interface{}) (*Scenario, error)
	DeleteScenario(scenarioID int) error
	ValidateScenarioFormat(scenario *Scenario) (bool, string)
	BulkCreateScenariosFromExcel(testID int, excelData [][]string) (created int, failures []string, err error)

	// Phase 2 - Advanced Management
	DuplicateScenario(scenarioID, newTestID int) (*Scenario, error)
	ReorderScenarios(testID int, scenarioOrder []int) error
	BulkUpdateScenarios(testID int, updates map[string]interface{}) (success []int, failed []int, err error)

	// Phase 2 - Advanced Retrieval & Filtering
	GetScenariosWithExecutionHistory(testID int, limit int) ([]ScenarioWithHistory, error)
	SearchScenarios(testID int, searchCriteria map[string]interface{}) ([]Scenario, error)
	GetScenariosByStatus(testID int, statusFilter string) ([]Scenario, error)

	// Phase 2 - Analytics & Performance
	GetScenarioExecutionStats(scenarioID int, timeRange string) (*ScenarioStats, error)
	GetScenariosBySuccessRate(testID int, threshold float64) ([]Scenario, error)
	GenerateScenarioReport(testID int, reportType string) (interface{}, error)

	// Phase 2 - Advanced Validation & Health
	ValidateScenarioExecutability(scenarioID int) (bool, string)
	ValidateScenarioSet(testID int) (bool, []string)
	CheckScenarioDependencies(scenarioID int) (map[string]interface{}, error)

	// Phase 2 - Metadata & Enrichment
	UpdateScenarioExecutionMetadata(scenarioID int, executionData map[string]interface{}) error
	TagScenarios(scenarioIDs []int, tags []string) error
	GetScenarioSuggestions(testID int, context map[string]interface{}) ([]Scenario, error)

	// Phase 2 - Import/Export
	ExportScenariosToExcel(testID int, format string) ([]byte, error)
	ImportScenariosWithValidation(testID int, source interface{}, validationRules map[string]interface{}) (summary interface{}, err error)
}

// Phase 2: Advanced types

type ScenarioWithHistory struct {
	Scenario
	LastRunDate    string
	SuccessRate    float64
	ExecutionStats map[string]interface{}
}

type ScenarioStats struct {
	ScenarioID         int
	SuccessRate        float64
	AverageExecTimeSec float64
	FailurePatterns    []string
	ConfidenceScores   []float64
	History            []map[string]interface{}
}

type ScenarioRepository struct {
	db *sql.DB
}

func NewScenarioRepository(db *sql.DB) ScenarioRepo {
	return &ScenarioRepository{db: db}
}

// CreateScenario creates a new scenario for a specific test.
func (r *ScenarioRepository) CreateScenario(testID int, description, expectedOutput string) (*Scenario, error) {
	intID := strconv.Itoa(testID)
	scenario := &Scenario{TestID: intID, Description: description, ExpectedOutput: expectedOutput, Status: "not_run"}
	if valid, msg := r.ValidateScenarioFormat(scenario); !valid {
		return nil, fmt.Errorf("invalid scenario: %s", msg)
	}
	var exists int
	err := r.db.QueryRow("SELECT COUNT(1) FROM tests WHERE id = ?", testID).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, fmt.Errorf("test with id %d does not exist", testID)
	}
	res, err := r.db.Exec("INSERT INTO scenarios (test_id, description, expected_output, status) VALUES (?, ?, ?, ?)", testID, description, expectedOutput, scenario.Status)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	scenario.ID = strconv.Itoa(int(id))
	return scenario, nil
}

// GetScenariosByTestID fetches all scenarios for a test, ordered by creation.
func (r *ScenarioRepository) GetScenariosByTestID(testID int) ([]Scenario, error) {
	rows, err := r.db.Query("SELECT id, test_id, description, expected_output, status FROM scenarios WHERE test_id = ? ORDER BY id ASC", testID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var scenarios []Scenario
	for rows.Next() {
		var s Scenario
		if err := rows.Scan(&s.ID, &s.TestID, &s.Description, &s.ExpectedOutput, &s.Status); err != nil {
			return nil, err
		}
		scenarios = append(scenarios, s)
	}
	return scenarios, nil
}

// GetScenarioByID retrieves a scenario by its ID.
func (r *ScenarioRepository) GetScenarioByID(scenarioID int) (*Scenario, error) {
	var s Scenario
	err := r.db.QueryRow("SELECT id, test_id, description, expected_output, status FROM scenarios WHERE id = ?", scenarioID).Scan(&s.ID, &s.TestID, &s.Description, &s.ExpectedOutput, &s.Status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// UpdateScenario updates scenario fields. Only description and expected_output are updatable.
func (r *ScenarioRepository) UpdateScenario(scenarioID int, updates map[string]interface{}) (*Scenario, error) {
	s, err := r.GetScenarioByID(scenarioID)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("scenario with id %d not found", scenarioID)
	}
	if desc, ok := updates["description"].(string); ok {
		s.Description = desc
	}
	if eo, ok := updates["expected_output"].(string); ok {
		s.ExpectedOutput = eo
	}
	if status, ok := updates["status"].(string); ok {
		s.Status = status
	}
	if valid, msg := r.ValidateScenarioFormat(s); !valid {
		return nil, fmt.Errorf("invalid scenario: %s", msg)
	}
	_, err = r.db.Exec("UPDATE scenarios SET description = ?, expected_output = ?, status = ? WHERE id = ?", s.Description, s.ExpectedOutput, s.Status, scenarioID)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// DeleteScenario removes a scenario by ID.
func (r *ScenarioRepository) DeleteScenario(scenarioID int) error {
	// Check existence
	s, err := r.GetScenarioByID(scenarioID)
	if err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("scenario with id %d not found", scenarioID)
	}
	_, err = r.db.Exec("DELETE FROM scenarios WHERE id = ?", scenarioID)
	return err
}

// ValidateScenarioFormat checks scenario requirements.
func (r *ScenarioRepository) ValidateScenarioFormat(scenario *Scenario) (bool, string) {
	descLen := len(scenario.Description)
	eoLen := len(scenario.ExpectedOutput)
	if descLen == 0 {
		return false, "description is required"
	}
	if eoLen == 0 {
		return false, "expected output is required"
	}
	if descLen < 2 || descLen > 500 {
		return false, "description must be 2-500 characters"
	}
	if eoLen < 2 || eoLen > 500 {
		return false, "expected output must be 2-500 characters"
	}
	return true, ""
}

// BulkCreateScenariosFromExcel imports scenarios from Excel-like data.
func (r *ScenarioRepository) BulkCreateScenariosFromExcel(testID int, excelData [][]string) (created int, failures []string, err error) {
	if len(excelData) < 2 {
		return 0, nil, fmt.Errorf("no data rows found")
	}
	headers := excelData[0]
	descIdx, eoIdx := -1, -1
	for i, h := range headers {
		if h == "description" {
			descIdx = i
		}
		if h == "expected_output" {
			eoIdx = i
		}
	}
	if descIdx == -1 || eoIdx == -1 {
		return 0, nil, fmt.Errorf("missing required columns: description, expected_output")
	}
	for i, row := range excelData[1:] {
		if len(row) <= descIdx || len(row) <= eoIdx {
			failures = append(failures, fmt.Sprintf("row %d: missing columns", i+2))
			continue
		}
		desc := row[descIdx]
		eo := row[eoIdx]
		_, err := r.CreateScenario(testID, desc, eo)
		if err != nil {
			failures = append(failures, fmt.Sprintf("row %d: %v", i+2, err))
			continue
		}
		created++
	}
	return created, failures, nil
}

// Phase 2: Advanced method stubs

func (r *ScenarioRepository) DuplicateScenario(scenarioID, newTestID int) (*Scenario, error) {
	// TODO: Implement duplication logic
	return nil, nil
}

func (r *ScenarioRepository) ReorderScenarios(testID int, scenarioOrder []int) error {
	// TODO: Implement reorder logic
	return nil
}

func (r *ScenarioRepository) BulkUpdateScenarios(testID int, updates map[string]interface{}) (success []int, failed []int, err error) {
	// TODO: Implement bulk update logic
	return nil, nil, nil
}

func (r *ScenarioRepository) GetScenariosWithExecutionHistory(testID int, limit int) ([]ScenarioWithHistory, error) {
	// TODO: Implement retrieval with execution history
	return nil, nil
}

func (r *ScenarioRepository) SearchScenarios(testID int, searchCriteria map[string]interface{}) ([]Scenario, error) {
	// TODO: Implement search logic
	return nil, nil
}

func (r *ScenarioRepository) GetScenariosByStatus(testID int, statusFilter string) ([]Scenario, error) {
	// TODO: Implement status filter logic
	return nil, nil
}

func (r *ScenarioRepository) GetScenarioExecutionStats(scenarioID int, timeRange string) (*ScenarioStats, error) {
	// TODO: Implement scenario stats retrieval
	return nil, nil
}

func (r *ScenarioRepository) GetScenariosBySuccessRate(testID int, threshold float64) ([]Scenario, error) {
	// TODO: Implement success rate filter
	return nil, nil
}

func (r *ScenarioRepository) GenerateScenarioReport(testID int, reportType string) (interface{}, error) {
	// TODO: Implement scenario report generation
	return nil, nil
}

func (r *ScenarioRepository) ValidateScenarioExecutability(scenarioID int) (bool, string) {
	// TODO: Implement executability validation
	return true, ""
}

func (r *ScenarioRepository) ValidateScenarioSet(testID int) (bool, []string) {
	// TODO: Implement scenario set validation
	return true, nil
}

func (r *ScenarioRepository) CheckScenarioDependencies(scenarioID int) (map[string]interface{}, error) {
	// TODO: Implement dependency check
	return nil, nil
}

func (r *ScenarioRepository) UpdateScenarioExecutionMetadata(scenarioID int, executionData map[string]interface{}) error {
	// TODO: Implement metadata update
	return nil
}

func (r *ScenarioRepository) TagScenarios(scenarioIDs []int, tags []string) error {
	// TODO: Implement tagging
	return nil
}

func (r *ScenarioRepository) GetScenarioSuggestions(testID int, context map[string]interface{}) ([]Scenario, error) {
	// TODO: Implement AI-powered suggestions
	return nil, nil
}

func (r *ScenarioRepository) ExportScenariosToExcel(testID int, format string) ([]byte, error) {
	// TODO: Implement export to Excel
	return nil, nil
}

func (r *ScenarioRepository) ImportScenariosWithValidation(testID int, source interface{}, validationRules map[string]interface{}) (summary interface{}, err error) {
	// TODO: Implement advanced import with validation
	return nil, nil
}
