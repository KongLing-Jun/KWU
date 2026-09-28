package project

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func requireNonNullProjectFields(document map[string]json.RawMessage, fields []string, label string) error {
	for _, field := range fields {
		value, ok := document[field]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("projects-v1: %s field %s must be present and non-null", label, field)
		}
	}
	return nil
}

func validateNestedProjectFields(document map[string]json.RawMessage, field string, allowed []string, label string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(document[field], &object); err != nil {
		return fmt.Errorf("projects-v1: %s %s must be an object", label, field)
	}
	if err := requireExactProjectFields(object, allowed...); err != nil {
		return fmt.Errorf("projects-v1: %s %s: %w", label, field, err)
	}
	return nil
}
