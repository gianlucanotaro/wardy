package store

import (
	"database/sql"
	"encoding/json"
)

// NullFloat64 is sql.NullFloat64 with clean JSON: null when absent, the
// bare number otherwise — same rationale as NullString.
type NullFloat64 sql.NullFloat64

func (n NullFloat64) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.Float64)
}

func (n *NullFloat64) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*n = NullFloat64{}
		return nil
	}
	if err := json.Unmarshal(data, &n.Float64); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

func (n *NullFloat64) Scan(value any) error {
	return (*sql.NullFloat64)(n).Scan(value)
}
