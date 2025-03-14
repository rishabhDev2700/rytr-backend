package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"rytr/internal/database/models"
	"rytr/internal/utils"

	"github.com/google/uuid"
)

type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	// have to be used later
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id uuid.UUID) error
	ResetPassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error
}

type userRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (first_name, last_name, email, password, created_at, updated_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id, created_at, updated_at`
	err := r.db.QueryRowContext(ctx, query, user.FirstName, user.LastName, user.Email, user.Password).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("error creating user: %v", err)
	}
	return nil
}

func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user := models.User{}
	query := `SELECT id, first_name, last_name, email, created_at, updated_at FROM users where id = $1`
	err := r.db.QueryRowContext(ctx, query).Scan(user.ID, user.FirstName, user.LastName, user.Email, user.CreatedAt, user.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("error getting user: %v", err)
	}
	return &user, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	user := models.User{}
	query := `SELECT id, first_name, last_name, email, password, created_at, updated_at FROM users where email = $1`
	err := r.db.QueryRowContext(ctx, query, email).Scan(&user.ID, &user.FirstName, &user.LastName, &user.Email, &user.Password, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("error getting user: %v", err)
	}
	return &user, nil
}

func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	return nil
}

func (r *userRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (r *userRepository) ResetPassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error {
	// First verify that the old password matches
	var storedPasswordHash string
	query := `SELECT password FROM users WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, userID).Scan(&storedPasswordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("user not found")
		}
		return fmt.Errorf("failed to retrieve user password: %w", err)
	}

	// Verify old password using the CheckPasswordHash utility
	if !utils.CheckPasswordHash(oldPassword, storedPasswordHash) {
		return errors.New("incorrect password")
	}

	// Hash the new password
	hashedNewPassword, err := utils.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	// Update to new password if old password is correct
	updateQuery := `UPDATE users SET password = $1 WHERE id = $2`

	result, err := r.db.ExecContext(ctx, updateQuery, hashedNewPassword, userID)
	if err != nil {
		return fmt.Errorf("failed to reset password: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("password update failed")
	}

	return nil
}
