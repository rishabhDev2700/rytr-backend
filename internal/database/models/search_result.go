package models
type SearchResult struct {
   Notes []Note `json:"notes"`
   Cards []Card `json:"cards"`
}

