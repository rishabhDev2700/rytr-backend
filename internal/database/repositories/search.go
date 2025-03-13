package repositories

import (
	"context"
	"database/sql"
	"rytr/internal/database/models"
	"strings"

	"github.com/google/uuid"
)

type SearchRepository interface {
	SearchQuery(ctx context.Context, query string, userID uuid.UUID) (*models.SearchResult, error)
}

type searchRepository struct {
	db *sql.DB
}

func NewSearchRepository(db *sql.DB) SearchRepository {
	return &searchRepository{db: db}
}

func (s *searchRepository) SearchQuery(ctx context.Context, query string, userID uuid.UUID) (*models.SearchResult, error) {
	tsQuery := "to_tsquery('english', $1)"
	notesQuery := `
   	SELECT id, title, content, created_at, updated_at, user_id
   	FROM notes
   	WHERE user_id = $2 AND 
   	      (to_tsvector('english', title) @@ ` + tsQuery + ` OR 
   	       to_tsvector('english', content) @@ ` + tsQuery + `)
   	ORDER BY ts_rank(to_tsvector('english', title || ' ' || content), ` + tsQuery + `) DESC
   `
	cardsQuery := `
   	SELECT id, title, description, status, created_at, updated_at, user_id
   	FROM cards
   	WHERE user_id = $2 AND 
   	      (to_tsvector('english', title) @@ ` + tsQuery + ` OR 
   	       to_tsvector('english', description) @@ ` + tsQuery + `)
   	ORDER BY ts_rank(to_tsvector('english', title || ' ' || description), ` + tsQuery + `) DESC
   `

	formattedQuery := formatTsQuery(query)

	notesRows, err := s.db.QueryContext(ctx, notesQuery, formattedQuery, userID)
	if err != nil {
		return &models.SearchResult{}, err
	}
	defer notesRows.Close()

	var notes []models.Note
	for notesRows.Next() {
		var note models.Note
		if err := notesRows.Scan(
			&note.ID,
			&note.Title,
			&note.Content,
			&note.CreatedAt,
			&note.UpdatedAt,
			&note.UserID,
		); err != nil {
			return &models.SearchResult{}, err
		}
		notes = append(notes, note)
	}

	if err := notesRows.Err(); err != nil {
		return &models.SearchResult{}, err
	}

	cardsRows, err := s.db.QueryContext(ctx, cardsQuery, formattedQuery, userID)
	if err != nil {
		return &models.SearchResult{}, err
	}
	defer cardsRows.Close()

	var cards []models.Card
	for cardsRows.Next() {
		var card models.Card
		if err := cardsRows.Scan(
			&card.ID,
			&card.Title,
			&card.Description,
			&card.Status,
			&card.CreatedAt,
			&card.UpdatedAt,
			&card.UserID,
		); err != nil {
			return &models.SearchResult{}, err
		}
		cards = append(cards, card)
	}

	if err := cardsRows.Err(); err != nil {
		return &models.SearchResult{}, err
	}
	return &models.SearchResult{
		Notes: notes,
		Cards: cards,
	}, nil
}
func formatTsQuery(query string) string {
	// Split the query into words
	words := strings.Fields(query)

	// Process each word
	for i, word := range words {
		// Escape special characters
		word = strings.ReplaceAll(word, "'", "''")
		// Add prefix matching
		words[i] = word + ":*"
	}

	// Join with & for AND operations
	return strings.Join(words, " & ")
}
