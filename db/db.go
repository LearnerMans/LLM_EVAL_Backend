package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

func InitDB() {
	const dbPath = "./db.db"

	// Check if the DB file already exists
	_, err := os.Stat(dbPath)
	dbExists := !os.IsNotExist(err)

	if dbExists {
		fmt.Println("Database file already exists.")
	} else {
		fmt.Println("Database file does not exist. It will be created.")
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	schema := `
	CREATE TABLE IF NOT EXISTS tests (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		tenant_id TEXT NOT NULL,
		project_id TEXT NOT NULL,
		max_interactions INTEGER DEFAULT 10,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS scenarios (
		id INTEGER PRIMARY KEY,
		test_id INTEGER,
		description TEXT NOT NULL,
		expected_output TEXT,
		status TEXT DEFAULT 'Not Run',
		started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		completed_at DATETIME,
		FOREIGN KEY (test_id) REFERENCES tests(id)
	);

		CREATE TABLE IF NOT EXISTS runs (
		id INTEGER PRIMARY KEY,
		scenario_id INTEGER,
		status TEXT DEFAULT 'Not Run',
		started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		completed_at DATETIME,
		verdict TEXT,
		verdict_reasoning TEXT,
		prompt TEXT,
		tester_model TEXT,
		tested_model TEXT,
		FOREIGN KEY (scenario_id) REFERENCES scenarios(id)
	);
	
	CREATE TABLE IF NOT EXISTS interactions (
		id INTEGER PRIMARY KEY,
		run_id INTEGER,
		scenario_id INTEGER,
		turn_number INTEGER,
		user_message TEXT,
		llm_response TEXT,
		evaluation_result TEXT,
		evaluation_reasoning TEXT,
		FOREIGN KEY (run_id) REFERENCES runs(id)
	);`

	_, err = db.Exec(schema)
	if err != nil {
		log.Fatalf("Error creating tables: %v", err)
	}

	fmt.Println("Database and tables are ready!")
}

// ConnectDB establishes and returns a connection to the SQLite database.
func ConnectDB() (*sql.DB, error) {
	const dbPath = "./db.db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}
	// Optionally, ping to check connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}
	return db, nil
}
