package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// StringSlice maps a Go []string to a MySQL JSON column.
type StringSlice []string

func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = StringSlice{}
		return nil
	}
	bytes, ok := toBytes(value)
	if !ok {
		return errors.New("failed to scan StringSlice: unsupported type")
	}
	if len(bytes) == 0 {
		*s = StringSlice{}
		return nil
	}
	return json.Unmarshal(bytes, s)
}

// Uint64Slice maps a Go []uint64 to a MySQL JSON column.
type Uint64Slice []uint64

func (s Uint64Slice) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (s *Uint64Slice) Scan(value interface{}) error {
	if value == nil {
		*s = Uint64Slice{}
		return nil
	}
	bytes, ok := toBytes(value)
	if !ok {
		return errors.New("failed to scan Uint64Slice: unsupported type")
	}
	if len(bytes) == 0 {
		*s = Uint64Slice{}
		return nil
	}
	return json.Unmarshal(bytes, s)
}

// JSONRaw maps arbitrary JSON (object or array) to a MySQL JSON column.
type JSONRaw json.RawMessage

func (j JSONRaw) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

func (j *JSONRaw) UnmarshalJSON(data []byte) error {
	if j == nil {
		return errors.New("JSONRaw: UnmarshalJSON on nil pointer")
	}
	*j = append((*j)[0:0], data...)
	return nil
}

func (j JSONRaw) Value() (driver.Value, error) {
	if len(j) == 0 || string(j) == "null" {
		return nil, nil
	}
	return string(j), nil
}

func (j *JSONRaw) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := toBytes(value)
	if !ok {
		return errors.New("failed to scan JSONRaw: unsupported type")
	}
	*j = append((*j)[0:0], bytes...)
	return nil
}

func toBytes(value interface{}) ([]byte, bool) {
	switch v := value.(type) {
	case []byte:
		return v, true
	case string:
		return []byte(v), true
	default:
		return nil, false
	}
}
