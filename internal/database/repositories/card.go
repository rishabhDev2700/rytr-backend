package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"rytr/internal/database/models"

	"github.com/google/uuid"
)

type CardRepository interface {
	Create(ctx context.Context, Card *models.Card) error
	GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*models.Card, error)
	GetAll(ctx context.Context, id uuid.UUID) (*[]models.Card, error)
	GetPending(ctx context.Context, id uuid.UUID) (*[]models.Card, error)
	Update(ctx context.Context, Card *models.Card, userID uuid.UUID) error
	UpdateStatus(ctx context.Context, cardID uuid.UUID, status int8, userID uuid.UUID) error
	//have to be used
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
}

type cardRepository struct {
	db *sql.DB
}

func NewCardRepository(db *sql.DB) CardRepository {
	return &cardRepository{db: db}
}

func (r *cardRepository) Create(ctx context.Context, card *models.Card) error {
	query := `
		INSERT INTO cards (title, description, status, user_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id, created_at, updated_at`
	err := r.db.QueryRowContext(ctx, query, card.Title, card.Description, card.Status, card.UserID).Scan(&card.ID, &card.CreatedAt, &card.UpdatedAt)
	if err != nil {
		return fmt.Errorf("error creating card: %v", err)
	}
	return nil
}

func (r *cardRepository) GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*models.Card, error) {
	card := models.Card{}
	query := `SELECT id, title, description, status, user_id, created_at, updated_at FROM cards WHERE id = $1 AND user_id = $2`
	err := r.db.QueryRowContext(ctx, query, id, userID).Scan(&card.ID, &card.Title, &card.Description, &card.Status, &card.UserID, &card.CreatedAt, &card.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, errors.New("card not found")
	}
	if err != nil {
		return nil, fmt.Errorf("error getting card: %v", err)
	}
	return &card, nil
}
func (r *cardRepository) GetAll(ctx context.Context, id uuid.UUID) (*[]models.Card, error) {
	query := `SELECT id, title, description, status, user_id, created_at,updated_at FROM cards where user_id = $1`
	result, err := r.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("error querying cards: %v", err)
	}
	defer result.Close()
	var cards []models.Card
	for result.Next() {
		var card models.Card
		err := result.Scan(
			&card.ID,
			&card.Title,
			&card.Description,
			&card.Status,
			&card.UserID,
			&card.CreatedAt,
			&card.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning card: %v", err)
		}
		cards = append(cards, card)
	}
	if err = result.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %v", err)
	}
	return &cards, nil
}

func (r *cardRepository) GetPending(ctx context.Context, id uuid.UUID) (*[]models.Card, error) {
	query := `SELECT id, title, description, status, user_id, created_at,updated_at FROM cards where user_id = $1 AND status=1`
	result, err := r.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("error querying cards: %v", err)
	}
	defer result.Close()
	var cards []models.Card
	for result.Next() {
		var card models.Card
		err := result.Scan(
			&card.ID,
			&card.Title,
			&card.Description,
			&card.Status,
			&card.UserID,
			&card.CreatedAt,
			&card.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning card: %v", err)
		}
		cards = append(cards, card)
	}
	if err = result.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %v", err)
	}
	return &cards, nil
}


func (r *cardRepository) Update(ctx context.Context, card *models.Card, userID uuid.UUID) error {
	query := `
		UPDATE cards
		SET title = $1, description = $2, status = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $4 AND user_id = $5
		RETURNING updated_at`
	result, err := r.db.ExecContext(ctx, query, card.Title, card.Description, card.Status, card.ID, userID)
	if err != nil {
		return fmt.Errorf("error updating user: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return errors.New("user not found")
	}

	return nil
}

func (r *cardRepository) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	query := `DELETE FROM cards WHERE id = $1 and user_id = $2`
	result, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("error deleting card: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return errors.New("card not found")
	}
	return nil
}

func (r *cardRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status int8, userID uuid.UUID) error {

	query := `
		UPDATE cards
		SET status = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2 AND user_id = $3
		RETURNING status,updated_at`
	result, err := r.db.ExecContext(ctx, query, status, id, userID)
	if err != nil {
		return err
	}
	num, err := result.RowsAffected()
	fmt.Println("Rows affected:", num)
	return nil
}
