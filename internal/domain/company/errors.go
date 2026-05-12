package company

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNotFound      = errors.New("company not found")
	ErrDuplicateID   = errors.New("company id already exists")
	ErrDuplicateName = errors.New("company name already exists")
	ErrEmptyPatch    = errors.New("empty patch")
)

type ValidationError struct {
	Fields map[string]string
}

func (e ValidationError) Error() string {
	if len(e.Fields) == 0 {
		return "validation failed"
	}

	parts := make([]string, 0, len(e.Fields))
	for field, message := range e.Fields {
		parts = append(parts, fmt.Sprintf("%s: %s", field, message))
	}

	return "validation failed: " + strings.Join(parts, "; ")
}

func NewValidationError(fields map[string]string) error {
	if len(fields) == 0 {
		return nil
	}

	return ValidationError{Fields: fields}
}
