package actions

import pg "github.com/go-jet/jet/v2/postgres"

type SortDirection string

const (
	SortASC  SortDirection = "asc"
	SortDESC SortDirection = "desc"
)

type SortOption struct {
	Column    pg.Column     `json:"-"`
	Direction SortDirection `json:"-"`
}

type SortResponseItem struct {
	Value string `json:"value"`
	Text  string `json:"text"`
}

// SortActiveResponse names the order a list was actually served in, so a
// consumer that adds a row to a loaded page can put it where the list would
// have put it instead of at the front.
type SortActiveResponse struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}
