package store

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/gianlucanotaro/wardy/internal/images"
)

// MaybeBox is an item's garment box (images.Box) when it has been measured:
// the empty string in the database, and null in JSON, mean it has not.
type MaybeBox struct {
	Box   images.Box
	Valid bool
}

func (m MaybeBox) Value() (driver.Value, error) {
	if !m.Valid {
		return "", nil
	}
	b, err := json.Marshal(m.Box)
	if err != nil {
		return nil, fmt.Errorf("marshal mask box: %w", err)
	}
	return string(b), nil
}

func (m *MaybeBox) Scan(value any) error {
	var raw []byte
	switch v := value.(type) {
	case nil:
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("scan mask box: unsupported type %T", value)
	}
	if len(raw) == 0 {
		*m = MaybeBox{}
		return nil
	}
	if err := json.Unmarshal(raw, &m.Box); err != nil {
		return fmt.Errorf("scan mask box: %w", err)
	}
	m.Valid = true
	return nil
}

func (m MaybeBox) MarshalJSON() ([]byte, error) {
	if !m.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(m.Box)
}

func (m *MaybeBox) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*m = MaybeBox{}
		return nil
	}
	if err := json.Unmarshal(data, &m.Box); err != nil {
		return err
	}
	m.Valid = true
	return nil
}
