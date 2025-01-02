package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"rytr/internal/database/models"

	"github.com/google/uuid"
)

type NoteRepository interface {
	Create(ctx context.Context, Note *models.Note) error
	GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*models.Note, error)
	GetAll(ctx context.Context, userID uuid.UUID) (*[]models.Note, error)
	Update(ctx context.Context, Note *models.Note, userID uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
}

type noteRepository struct {
	db *sql.DB
}

func NewNoteRepository(db *sql.DB) NoteRepository {
	return &noteRepository{db: db}
}

func (r *noteRepository) Create(ctx context.Context, note *models.Note) error {
	query := `
		INSERT INTO notes (title, content, user_id, created_at, updated_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id, created_at, updated_at`
	err := r.db.QueryRowContext(ctx, query, note.Title, note.Content, note.UserID).Scan(&note.ID, &note.CreatedAt, &note.UpdatedAt)
	if err != nil {
		return fmt.Errorf("error creating card: %v", err)
	}
	return nil
}

func (r *noteRepository) GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*models.Note, error) {
	note := models.Note{}
	query := `SELECT id, title, content, user_id, created_at, updated_at FROM notes WHERE id = $1 AND user_id = $2`
	err := r.db.QueryRowContext(ctx, query, id, userID).Scan(&note.ID, &note.Title, &note.Content, &note.UserID, &note.CreatedAt, &note.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, errors.New("note not found")
	}
	if err != nil {
		return nil, fmt.Errorf("error getting note: %v", err)
	}
	return &note, nil
}

func (r *noteRepository) GetAll(ctx context.Context, userID uuid.UUID) (*[]models.Note, error) {
	query := `SELECT id, title, content, user_id, created_at,updated_at FROM notes where user_id = $1`
	result, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("error querying notes: %v", err)
	}
	defer result.Close()
	var notes []models.Note
	for result.Next() {
		var note models.Note
		err := result.Scan(
			&note.ID,
			&note.Title,
			&note.Content,
			&note.UserID,
			&note.CreatedAt,
			&note.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning note: %v", err)
		}
		notes = append(notes, note)
	}
	if err = result.Err(); err != nil {
		return nil, fmt.Errorf("error iterating notes: %v", err)
	}
	return &notes, nil

}

func (r *noteRepository) Update(ctx context.Context, note *models.Note, userID uuid.UUID) error {
	query := `
			UPDATE notes
			SET title = $1, content = $2, updated_at = CURRENT_TIMESTAMP
			WHERE id = $3 AND user_id = $4`
	result, err := r.db.ExecContext(ctx, query, note.Title, note.Content, note.ID, userID)
	if err != nil {
		return fmt.Errorf("error updating note: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		return errors.New("no rows updated")
	}
	return nil
}

func (r *noteRepository) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	query := `DELETE FROM notes WHERE id = $1 AND user_id = $2`
	result, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("error deleting note: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		return errors.New("no rows deleted")
	}

	return nil
}
