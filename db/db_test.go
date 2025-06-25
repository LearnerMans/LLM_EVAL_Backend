package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// TestInitDB_CreationAndSchema checks if InitDB creates the DB file and the correct tables.
func TestInitDB_CreationAndSchema(t *testing.T) {
	tempDir := t.TempDir() // Creates a temporary directory that is automatically cleaned up
	dbPath := filepath.Join(tempDir, "test_db.db")

	// Override the dbPath used in InitDB. This requires InitDB to be adaptable or the test to manage the path.
	// For this test, let's assume InitDB can be modified to accept a path, or we modify the global const for testing.
	// Since dbPath is a const in db.go, we can't easily change it without modifying db.go.
	// Option 1: Modify db.go to use a variable for dbPath.
	// Option 2: Replicate InitDB's logic here with a configurable path (less ideal for testing the actual function).
	// Option 3: Use the default "db.db" and manage its creation/deletion carefully. (risky for parallel tests or existing user dbs)

	// Let's try to make db.go use a variable for dbPath for testability.
	// If not, this test will operate on "./db.db", which is not ideal.
	// For now, I will write the test assuming we *can* control the dbPath.
	// I will add a step to modify db.go to use a variable for dbPath.

	originalDbPath := GetDBPath() // Need a getter in db.go
	SetDBPath(dbPath)             // Need a setter in db.go
	defer func() {
		SetDBPath(originalDbPath) // Restore original path
		// The tempDir cleanup should handle the file, but explicit removal can be added if needed.
	}()

	// Ensure DB does not exist initially (it shouldn't in tempDir)
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("Database file %s already exists before test", dbPath)
	}

	InitDB() // Call the function to be tested

	// 1. Check if DB file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("InitDB did not create the database file at %s", dbPath)
	}

	// 2. Check if tables were created
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database at %s: %v", dbPath, err)
	}
	defer db.Close()

	expectedTables := []string{"tests", "scenarios", "test_runs", "interactions"}
	for _, tableName := range expectedTables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?;", tableName).Scan(&name)
		if err != nil {
			if err == sql.ErrNoRows {
				t.Errorf("Table %s was not created by InitDB", tableName)
			} else {
				t.Errorf("Error checking for table %s: %v", tableName, err)
			}
		}
	}

	// 3. Check for idempotency: Run InitDB again
	InitDB() // Call again

	// Re-check tables (a simple check, could be more thorough by verifying schema details if needed)
	for _, tableName := range expectedTables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?;", tableName).Scan(&name)
		if err != nil {
			if err == sql.ErrNoRows {
				t.Errorf("Table %s was removed or not found after second InitDB call", tableName)
			} else {
				t.Errorf("Error checking for table %s after second InitDB call: %v", tableName, err)
			}
		}
	}
}

// TestInitDB_ExistingDB checks behavior when DB file already exists.
func TestInitDB_ExistingDB(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "existing_test_db.db")

	originalDbPath := GetDBPath()
	SetDBPath(dbPath)
	defer func() {
		SetDBPath(originalDbPath)
	}()

	// Create an empty DB file first
	file, err := os.Create(dbPath)
	if err != nil {
		t.Fatalf("Failed to create dummy empty DB file: %v", err)
	}
	file.Close()

	InitDB() // Call the function

	// Check if tables were created even if file existed
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database at %s: %v", dbPath, err)
	}
	defer db.Close()

	expectedTables := []string{"tests", "scenarios", "test_runs", "interactions"}
	for _, tableName := range expectedTables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?;", tableName).Scan(&name)
		if err != nil {
			t.Errorf("Table %s was not created when DB file already existed (err: %v)", tableName, err)
		}
	}
}

// Note: The above tests rely on GetDBPath() and SetDBPath() functions being added to db.go
// to make the database path configurable for testing.
// Example changes needed in db.go:
//
// var currentDbPath = "./db.db" // Default
//
// func GetDBPath() string {
//     return currentDbPath
// }
//
// func SetDBPath(newPath string) {
//     currentDbPath = newPath
// }
//
// // And in InitDB():
// // const dbPath = "./db.db" // Remove this
// // Use currentDbPath instead of the const dbPath
//
// I will add these changes to db.go.
