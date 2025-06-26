package repository

import (
	"database/sql"
)

type Interaction struct {
	ID                  int
	TestRunID           int
	ScenarioID          int
	TurnNumber          int
	UserMessage         string
	LLMResponse         string
	EvaluationResult    string
	EvaluationReasoning string
}

type InteractionRepository struct {
	db *sql.DB
}

func NewInteractionRepository(db *sql.DB) *InteractionRepository {
	return &InteractionRepository{db: db}
}

func (r *InteractionRepository) Create(interaction *Interaction) error {
	query := `INSERT INTO interactions (test_run_id, scenario_id, turn_number, user_message, llm_response, evaluation_result, evaluation_reasoning) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.Exec(query, interaction.TestRunID, interaction.ScenarioID, interaction.TurnNumber, interaction.UserMessage, interaction.LLMResponse, interaction.EvaluationResult, interaction.EvaluationReasoning)
	return err
}

func (r *InteractionRepository) GetByTestRunID(testRunID int) ([]Interaction, error) {
	query := `SELECT id, test_run_id, scenario_id, turn_number, user_message, llm_response, evaluation_result, evaluation_reasoning FROM interactions WHERE test_run_id = ?`
	rows, err := r.db.Query(query, testRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var interactions []Interaction
	for rows.Next() {
		var i Interaction
		if err := rows.Scan(&i.ID, &i.TestRunID, &i.ScenarioID, &i.TurnNumber, &i.UserMessage, &i.LLMResponse, &i.EvaluationResult, &i.EvaluationReasoning); err != nil {
			return nil, err
		}
		interactions = append(interactions, i)
	}
	return interactions, nil
}
