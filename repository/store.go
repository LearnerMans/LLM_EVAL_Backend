package repository

import (
	"database/sql"
)

type Store struct {
	Interaction *InteractionRepository
}

func NewStore(db *sql.DB) *Store {
	return &Store{
		Interaction: NewInteractionRepository(db),
	}
}
