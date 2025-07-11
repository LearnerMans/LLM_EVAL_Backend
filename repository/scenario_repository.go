package repository

import (
	"database/sql"
	"fmt"
	"log" // Added for debugging potential issues
	// "strconv" // No longer needed for ID conversions directly in these methods
)

// Scenario struct updated to use int for ID and TestID
type Scenario struct {
	ID             int    `json:"id"` // Add json tags for consistency if handlers marshal this directly
	TestID         int    `json:"test_id"`
	Description    string `json:"description"`
	ExpectedOutput string `json:"expected_output"`
	Status         string `json:"status"`
	// Add CreatedAt, UpdatedAt if they are in the table and needed
}

type ScenarioRepo interface {
	CreateScenario(testID int, description, expectedOutput string) (*Scenario, error)
	GetScenariosByTestID(testID int) ([]Scenario, error)
	GetScenarioByID(scenarioID int) (*Scenario, error)
	UpdateScenario(scenarioID int, updates map[string]interface{}) (*Scenario, error)
	DeleteScenario(scenarioID int) error
	ValidateScenarioFormat(scenario *Scenario) (bool, string)
	// ... other methods from the interface
	BulkCreateScenariosFromExcel(testID int, excelData [][]string) (created int, failures []string, err error)
	DuplicateScenario(scenarioID, newTestID int) (*Scenario, error)
	ReorderScenarios(testID int, scenarioOrder []int) error
	BulkUpdateScenarios(testID int, updates map[string]interface{}) (success []int, failed []int, err error)
	GetScenariosWithExecutionHistory(testID int, limit int) ([]ScenarioWithHistory, error)
	SearchScenarios(testID int, searchCriteria map[string]interface{}) ([]Scenario, error)
	GetScenariosByStatus(testID int, statusFilter string) ([]Scenario, error)
	GetScenarioExecutionStats(scenarioID int, timeRange string) (*ScenarioStats, error)
	GetScenariosBySuccessRate(testID int, threshold float64) ([]Scenario, error)
	GenerateScenarioReport(testID int, reportType string) (interface{}, error)
	ValidateScenarioExecutability(scenarioID int) (bool, string)
	ValidateScenarioSet(testID int) (bool, []string)
	CheckScenarioDependencies(scenarioID int) (map[string]interface{}, error)
	UpdateScenarioExecutionMetadata(scenarioID int, executionData map[string]interface{}) error
	TagScenarios(scenarioIDs []int, tags []string) error
	GetScenarioSuggestions(testID int, context map[string]interface{}) ([]Scenario, error)
	ExportScenariosToExcel(testID int, format string) ([]byte, error)
	ImportScenariosWithValidation(testID int, source interface{}, validationRules map[string]interface{}) (summary interface{}, err error)
}

// Phase 2: Advanced types (assuming these are okay for now)
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
	scenario := &Scenario{TestID: testID, Description: description, ExpectedOutput: expectedOutput, Status: "Not Run"} // Default status

	// Validate format before DB interaction
	if valid, msg := r.ValidateScenarioFormat(scenario); !valid {
		return nil, fmt.Errorf("invalid scenario format: %s", msg)
	}

	// Check if the parent test exists
	var exists int
	err := r.db.QueryRow("SELECT COUNT(1) FROM tests WHERE id = ?", testID).Scan(&exists)
	if err != nil {
		log.Printf("Error checking if test exists (TestID: %d): %v", testID, err)
		return nil, fmt.Errorf("database error checking test existence: %w", err)
	}
	if exists == 0 {
		return nil, fmt.Errorf("test with id %d does not exist", testID)
	}

	// Insert the scenario
	stmt, err := r.db.Prepare("INSERT INTO scenarios (test_id, description, expected_output, status) VALUES (?, ?, ?, ?)")
	if err != nil {
		log.Printf("Error preparing insert statement for scenario (TestID: %d): %v", testID, err)
		return nil, fmt.Errorf("database error preparing insert statement: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.Exec(testID, description, expectedOutput, scenario.Status)
	if err != nil {
		log.Printf("Error executing insert statement for scenario (TestID: %d): %v", testID, err)
		return nil, fmt.Errorf("database error executing insert statement: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		log.Printf("Error getting last insert ID for scenario (TestID: %d): %v", testID, err)
		return nil, fmt.Errorf("database error getting last insert ID: %w", err)
	}
	scenario.ID = int(id) // Assign the auto-generated ID

	// Optionally, you might want to fetch the created_at timestamp if your table has it and struct needs it.
	// For now, returning the scenario as is.
	return scenario, nil
}

// GetScenariosByTestID fetches all scenarios for a test, ordered by creation.
func (r *ScenarioRepository) GetScenariosByTestID(testID int) ([]Scenario, error) {
	rows, err := r.db.Query("SELECT id, test_id, description, expected_output, status FROM scenarios WHERE test_id = ? ORDER BY id ASC", testID)
	if err != nil {
		log.Printf("Error querying scenarios by TestID %d: %v", testID, err)
		return nil, fmt.Errorf("database error querying scenarios: %w", err)
	}
	defer rows.Close()

	var scenarios []Scenario
	for rows.Next() {
		var s Scenario
		// Ensure Scan targets match the Scenario struct field types (all are int or string now)
		if err := rows.Scan(&s.ID, &s.TestID, &s.Description, &s.ExpectedOutput, &s.Status); err != nil {
			log.Printf("Error scanning scenario row (TestID: %d): %v", testID, err)
			return nil, fmt.Errorf("database error scanning scenario row: %w", err)
		}
		scenarios = append(scenarios, s)
	}
	if err = rows.Err(); err != nil {
		log.Printf("Error after iterating scenario rows (TestID: %d): %v", testID, err)
		return nil, fmt.Errorf("database error iterating scenario rows: %w", err)
	}
	return scenarios, nil
}

// GetScenarioByID retrieves a scenario by its ID.
func (r *ScenarioRepository) GetScenarioByID(scenarioID int) (*Scenario, error) {
	var s Scenario
	err := r.db.QueryRow("SELECT id, test_id, description, expected_output, status FROM scenarios WHERE id = ?", scenarioID).
		Scan(&s.ID, &s.TestID, &s.Description, &s.ExpectedOutput, &s.Status)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Or a specific "not found" error if preferred by handlers
		}
		log.Printf("Error querying scenario by ID %d: %v", scenarioID, err)
		return nil, fmt.Errorf("database error querying scenario by ID: %w", err)
	}
	return &s, nil
}

// UpdateScenario updates scenario fields.
func (r *ScenarioRepository) UpdateScenario(scenarioID int, updates map[string]interface{}) (*Scenario, error) {
	// Fetch the current scenario to ensure it exists and to have a base for updates
	s, err := r.GetScenarioByID(scenarioID)
	if err != nil {
		// GetScenarioByID already logs, so just pass up the error or wrap if more context needed
		return nil, fmt.Errorf("failed to retrieve scenario %d for update: %w", scenarioID, err)
	}
	if s == nil { // Should be handled by GetScenarioByID returning sql.ErrNoRows, which err would capture
		return nil, fmt.Errorf("scenario with id %d not found for update", scenarioID)
	}

	// Apply updates to the fetched scenario struct
	// This helps in validating the complete struct before saving
	changed := false
	if desc, ok := updates["description"].(string); ok {
		if s.Description != desc {
			s.Description = desc
			changed = true
		}
	}
	if eo, ok := updates["expected_output"].(string); ok {
		if s.ExpectedOutput != eo {
			s.ExpectedOutput = eo
			changed = true
		}
	}
	if status, ok := updates["status"].(string); ok {
		if s.Status != status {
			s.Status = status
			changed = true
		}
	}

	if !changed { // No actual changes to persist
		return s, nil // Return the original scenario
	}

	// Validate the updated scenario format
	if valid, msg := r.ValidateScenarioFormat(s); !valid {
		return nil, fmt.Errorf("invalid scenario format after update: %s", msg)
	}

	// Persist changes to the database
	stmt, err := r.db.Prepare("UPDATE scenarios SET description = ?, expected_output = ?, status = ? WHERE id = ?")
	if err != nil {
		log.Printf("Error preparing update statement for scenario ID %d: %v", scenarioID, err)
		return nil, fmt.Errorf("database error preparing update statement: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(s.Description, s.ExpectedOutput, s.Status, scenarioID)
	if err != nil {
		log.Printf("Error executing update statement for scenario ID %d: %v", scenarioID, err)
		return nil, fmt.Errorf("database error executing update: %w", err)
	}

	return s, nil // Return the updated scenario struct
}

// DeleteScenario removes a scenario by ID.
func (r *ScenarioRepository) DeleteScenario(scenarioID int) error {
	// First, check if the scenario exists to provide a clearer "not found" error if necessary,
	// though Exec will return RowsAffected which can also indicate this.
	// GetScenarioByID also handles logging if there's a DB error during the check.
	existingScenario, err := r.GetScenarioByID(scenarioID)
	if err != nil {
		// This error is from the attempt to fetch, not necessarily "not found" yet.
		return fmt.Errorf("error checking existence of scenario %d before deletion: %w", scenarioID, err)
	}
	if existingScenario == nil {
		// sql.ErrNoRows was encountered by GetScenarioByID, meaning it's not found.
		return fmt.Errorf("scenario with id %d not found, cannot delete", scenarioID) // Or return nil if "delete non-existent" is fine
	}

	stmt, err := r.db.Prepare("DELETE FROM scenarios WHERE id = ?")
	if err != nil {
		log.Printf("Error preparing delete statement for scenario ID %d: %v", scenarioID, err)
		return fmt.Errorf("database error preparing delete statement: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.Exec(scenarioID)
	if err != nil {
		log.Printf("Error executing delete statement for scenario ID %d: %v", scenarioID, err)
		return fmt.Errorf("database error executing delete: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected for delete scenario ID %d: %v", scenarioID, err)
		// Not returning error here as delete might have succeeded. This is more for logging.
	}
	if rowsAffected == 0 {
		// This case should ideally be caught by the GetScenarioByID check above,
		// but as a safeguard, if no rows were deleted, it implies the scenario didn't exist.
		return fmt.Errorf("scenario with id %d not found or already deleted (0 rows affected)", scenarioID)
	}

	return nil
}

// ValidateScenarioFormat checks scenario requirements.
// This method remains largely the same as it operates on the struct fields.
func (r *ScenarioRepository) ValidateScenarioFormat(scenario *Scenario) (bool, string) {
	descLen := len(scenario.Description)
	eoLen := len(scenario.ExpectedOutput)
	if descLen == 0 {
		return false, "description is required"
	}
	if eoLen == 0 {
		return false, "expected output is required"
	}
	// Consider making min/max lengths constants
	if descLen < 2 || descLen > 500 {
		return false, "description must be 2-500 characters"
	}
	if eoLen < 2 || eoLen > 500 {
		return false, "expected output must be 2-500 characters"
	}
	// Validate Status if there's a predefined set of allowed statuses
	// e.g. if scenario.Status != "Not Run" && scenario.Status != "Ready" && ...
	return true, ""
}


// BulkCreateScenariosFromExcel imports scenarios from Excel-like data.
// This method needs to be updated to use the new Scenario struct with int IDs.
// However, the core logic of parsing excelData and calling CreateScenario remains.
// CreateScenario now returns *Scenario with int ID.
func (r *ScenarioRepository) BulkCreateScenariosFromExcel(testID int, excelData [][]string) (created int, failures []string, err error) {
	if len(excelData) < 2 { // Assuming first row is header
		return 0, nil, fmt.Errorf("no data rows found in Excel data (expected header + data)")
	}
	headers := excelData[0]
	descIdx, eoIdx := -1, -1
	for i, h := range headers {
		// Consider case-insensitive matching for headers
		if h == "description" { // strings.ToLower(h) == "description"
			descIdx = i
		}
		if h == "expected_output" { // strings.ToLower(h) == "expected_output"
			eoIdx = i
		}
	}
	if descIdx == -1 || eoIdx == -1 {
		return 0, nil, fmt.Errorf("missing required columns in Excel data: 'description' and/or 'expected_output'")
	}

	for i, row := range excelData[1:] { // Skip header row
		if len(row) <= descIdx || len(row) <= eoIdx { // Check if row has enough columns
			failures = append(failures, fmt.Sprintf("row %d: missing required columns data", i+2)) // i+2 for 1-based Excel row number
			continue
		}
		desc := row[descIdx]
		eo := row[eoIdx]

		// CreateScenario now handles validation internally
		_, err := r.CreateScenario(testID, desc, eo)
		if err != nil {
			failures = append(failures, fmt.Sprintf("row %d (description: %s): %v", i+2, desc, err))
			continue
		}
		created++
	}
	return created, failures, nil
}


// --- Stubs for Phase 2 methods ---
// These methods signatures might need adjustment if they use Scenario struct internally.
// For now, keeping them as is.

func (r *ScenarioRepository) DuplicateScenario(scenarioID, newTestID int) (*Scenario, error) {
	// TODO: Implement duplication logic. Fetch scenarioID, create new with newTestID.
	return nil, fmt.Errorf("DuplicateScenario not implemented")
}

func (r *ScenarioRepository) ReorderScenarios(testID int, scenarioOrder []int) error {
	// TODO: Implement reorder logic. This usually involves an 'order' column in DB.
	return fmt.Errorf("ReorderScenarios not implemented")
}

func (r *ScenarioRepository) BulkUpdateScenarios(testID int, updates map[string]interface{}) (success []int, failed []int, err error) {
	// TODO: Implement bulk update logic
	return nil, nil, fmt.Errorf("BulkUpdateScenarios not implemented")
}

func (r *ScenarioRepository) GetScenariosWithExecutionHistory(testID int, limit int) ([]ScenarioWithHistory, error) {
	// TODO: Implement retrieval with execution history
	return nil, fmt.Errorf("GetScenariosWithExecutionHistory not implemented")
}

func (r *ScenarioRepository) SearchScenarios(testID int, searchCriteria map[string]interface{}) ([]Scenario, error) {
	// TODO: Implement search logic
	return nil, fmt.Errorf("SearchScenarios not implemented")
}

func (r *ScenarioRepository) GetScenariosByStatus(testID int, statusFilter string) ([]Scenario, error) {
	// TODO: Implement status filter logic
	rows, err := r.db.Query("SELECT id, test_id, description, expected_output, status FROM scenarios WHERE test_id = ? AND status = ? ORDER BY id ASC", testID, statusFilter)
	if err != nil {
		log.Printf("Error querying scenarios by TestID %d and Status %s: %v", testID, statusFilter, err)
		return nil, fmt.Errorf("database error querying scenarios by status: %w", err)
	}
	defer rows.Close()

	var scenarios []Scenario
	for rows.Next() {
		var s Scenario
		if err := rows.Scan(&s.ID, &s.TestID, &s.Description, &s.ExpectedOutput, &s.Status); err != nil {
			log.Printf("Error scanning scenario row (TestID: %d, Status: %s): %v", testID, statusFilter, err)
			return nil, fmt.Errorf("database error scanning scenario row: %w", err)
		}
		scenarios = append(scenarios, s)
	}
	if err = rows.Err(); err != nil {
		log.Printf("Error after iterating scenario rows (TestID: %d, Status: %s): %v", testID, statusFilter, err)
		return nil, fmt.Errorf("database error iterating scenario rows: %w", err)
	}
	return scenarios, nil
}


func (r *ScenarioRepository) GetScenarioExecutionStats(scenarioID int, timeRange string) (*ScenarioStats, error) {
	return nil, fmt.Errorf("GetScenarioExecutionStats not implemented")
}

func (r *ScenarioRepository) GetScenariosBySuccessRate(testID int, threshold float64) ([]Scenario, error) {
	return nil, fmt.Errorf("GetScenariosBySuccessRate not implemented")
}

func (r *ScenarioRepository) GenerateScenarioReport(testID int, reportType string) (interface{}, error) {
	return nil, fmt.Errorf("GenerateScenarioReport not implemented")
}

func (r *ScenarioRepository) ValidateScenarioExecutability(scenarioID int) (bool, string) {
	return true, "ValidateScenarioExecutability not implemented"
}

func (r *ScenarioRepository) ValidateScenarioSet(testID int) (bool, []string) {
	return true, []string{"ValidateScenarioSet not implemented"}
}

func (r *ScenarioRepository) CheckScenarioDependencies(scenarioID int) (map[string]interface{}, error) {
	return nil, fmt.Errorf("CheckScenarioDependencies not implemented")
}

func (r *ScenarioRepository) UpdateScenarioExecutionMetadata(scenarioID int, executionData map[string]interface{}) error {
	return fmt.Errorf("UpdateScenarioExecutionMetadata not implemented")
}

func (r *ScenarioRepository) TagScenarios(scenarioIDs []int, tags []string) error {
	return fmt.Errorf("TagScenarios not implemented")
}

func (r *ScenarioRepository) GetScenarioSuggestions(testID int, context map[string]interface{}) ([]Scenario, error) {
	return nil, fmt.Errorf("GetScenarioSuggestions not implemented")
}

func (r *ScenarioRepository) ExportScenariosToExcel(testID int, format string) ([]byte, error) {
	return nil, fmt.Errorf("ExportScenariosToExcel not implemented")
}

func (r *ScenarioRepository) ImportScenariosWithValidation(testID int, source interface{}, validationRules map[string]interface{}) (summary interface{}, err error) {
	return nil, fmt.Errorf("ImportScenariosWithValidation not implemented")
}
