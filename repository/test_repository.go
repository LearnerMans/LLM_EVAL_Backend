package repository

import (
	"database/sql"
	"fmt"
)

type Test struct {
	ID              int
	Name            string
	TenantID        string
	ProjectID       string
	MaxInteractions int
	CreatedAt       string
}

type TestRepo interface {
	CreateTest(name, tenantID, projectID string, maxInteractions int) (int, error)
	GetTestByID(testID int) (*Test, error)
	GetTestsByTenant(tenantID string, limit, offset int) ([]Test, error)
	GetTestsByProject(tenantID, projectID string) ([]Test, error)
	UpdateTest(testID int, updates map[string]interface{}) error
	DeleteTest(testID int) error
	SearchTests(criteria map[string]interface{}) ([]Test, error)
	GetTestsWithRunStats(filter map[string]interface{}) ([]Test, error)
	ValidateTenantProjectPair(tenantID, projectID string) (bool, error)
}

type TestRepository struct {
	db *sql.DB
}

func NewTestRepository(db *sql.DB) TestRepo {
	return &TestRepository{db: db}
}

func (r *TestRepository) CreateTest(name, tenantID, projectID string, maxInteractions int) (int, error) {
	// Validate tenant-project pair (stub, always true for now)
	valid, err := r.ValidateTenantProjectPair(tenantID, projectID)
	if err != nil {
		return 0, err
	}
	if !valid {
		return 0, fmt.Errorf("invalid tenant-project pair")
	}
	if maxInteractions <= 0 {
		maxInteractions = 10
	}
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO tests (name, tenant_id, project_id, max_interactions) VALUES (?, ?, ?, ?)`, name, tenantID, projectID, maxInteractions)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(id), nil
}

func (r *TestRepository) ValidateTenantProjectPair(tenantID, projectID string) (bool, error) {
	// TODO: Implement actual validation logic
	return true, nil
}

func (r *TestRepository) GetTestByID(testID int) (*Test, error) {
	row := r.db.QueryRow(`SELECT id, name, tenant_id, project_id, max_interactions, created_at FROM tests WHERE id = ?`, testID)
	var t Test
	if err := row.Scan(&t.ID, &t.Name, &t.TenantID, &t.ProjectID, &t.MaxInteractions, &t.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (r *TestRepository) DeleteTest(testID int) error {
	// TODO: Implement delete test logic
	return nil
}

func (r *TestRepository) GetTestsByProject(tenantID, projectID string) ([]Test, error) {
	// TODO: Implement get tests by project logic
	return nil, nil
}

func (r *TestRepository) GetTestsByTenant(tenantID string, limit, offset int) ([]Test, error) {
	// TODO: Implement get tests by tenant logic
	return nil, nil
}

func (r *TestRepository) GetTestsWithRunStats(filter map[string]interface{}) ([]Test, error) {
	// TODO: Implement get tests with run stats logic
	return nil, nil
}

func (r *TestRepository) SearchTests(criteria map[string]interface{}) ([]Test, error) {
	// TODO: Implement search tests logic
	return nil, nil
}

func (r *TestRepository) UpdateTest(testID int, updates map[string]interface{}) error {
	// TODO: Implement update test logic
	return nil
}
