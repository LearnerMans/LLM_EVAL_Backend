package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
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

	db, err := sql.Open("sqlite3", dbPath)
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
		FOREIGN KEY (test_id) REFERENCES tests(id)
	);

	CREATE TABLE IF NOT EXISTS test_runs (
		id INTEGER PRIMARY KEY,
		test_id INTEGER,
		status TEXT DEFAULT 'running',
		started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		completed_at DATETIME,
		FOREIGN KEY (test_id) REFERENCES tests(id)
	);

	CREATE TABLE IF NOT EXISTS interactions (
		id INTEGER PRIMARY KEY,
		test_run_id INTEGER,
		scenario_id INTEGER,
		turn_number INTEGER,
		user_message TEXT,
		llm_response TEXT,
		evaluation_result TEXT,
		evaluation_reasoning TEXT,
		FOREIGN KEY (test_run_id) REFERENCES test_runs(id)
	);`

	_, err = db.Exec(schema)
	if err != nil {
		log.Fatalf("Error creating tables: %v", err)
	}

	fmt.Println("Database and tables are ready!")
}
