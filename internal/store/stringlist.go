package store

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringList stores a Go []string as a JSON array in a TEXT column — used
// for items.seasons and items.occasions.
type StringList []string

func (l StringList) Value() (driver.Value, error) {
	if l == nil {
		l = StringList{}
	}
	b, err := json.Marshal([]string(l))
	if err != nil {
		return nil, fmt.Errorf("marshal string list: %w", err)
	}
	return string(b), nil
}

func (l *StringList) Scan(value any) error {
	if value == nil {
		*l = StringList{}
		return nil
	}

	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("unsupported type for StringList: %T", value)
	}

	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("unmarshal string list: %w", err)
	}
	*l = out
	return nil
}
