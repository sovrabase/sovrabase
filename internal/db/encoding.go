package db

import "encoding/json"

// Marshal serializes a value to JSON bytes.
func Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Unmarshal deserializes JSON bytes into a value.
func Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// MarshalMap serializes a map to JSON bytes.
func MarshalMap(m map[string]interface{}) ([]byte, error) {
	return json.Marshal(m)
}

// UnmarshalMap deserializes JSON bytes into a map.
func UnmarshalMap(data []byte) (map[string]interface{}, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}
