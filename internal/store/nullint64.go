package store

import (
	"database/sql"
	"encoding/json"
)

// NullInt64 is sql.NullInt64 with clean JSON: null when absent, the bare
// number otherwise — same rationale as NullString.
type NullInt64 sql.NullInt64

func (n NullInt64) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.Int64)
}

func (n *NullInt64) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*n = NullInt64{}
		return nil
	}
	if err := json.Unmarshal(data, &n.Int64); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

func (n *NullInt64) Scan(value any) error {
	return (*sql.NullInt64)(n).Scan(value)
}
