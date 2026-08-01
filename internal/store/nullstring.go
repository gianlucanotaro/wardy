package store

import (
	"database/sql"
	"encoding/json"
)

// NullString is sql.NullString with clean JSON: null when absent, the bare
// string otherwise, instead of leaking {"String":"...","Valid":bool} across
// the API.
type NullString sql.NullString

func (n NullString) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.String)
}

func (n *NullString) Scan(value any) error {
	return (*sql.NullString)(n).Scan(value)
}
