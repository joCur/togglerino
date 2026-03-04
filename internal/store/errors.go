package store

import (
	"errors"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// IsNotFound reports whether an error represents a "not found" condition,
// either from ErrNotFound or a wrapped pgx.ErrNoRows.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, pgx.ErrNoRows)
}
