package store

import (
	"errors"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// ErrLastAdmin is returned when an operation would leave a project with no admin.
var ErrLastAdmin = errors.New("cannot remove the only project admin")

// ErrDuplicateAppUserID is returned when an app user ID is already claimed by another user.
var ErrDuplicateAppUserID = errors.New("app user ID already claimed by another user")

// ErrRoleInUse is returned when a role cannot be deleted because it is still referenced.
var ErrRoleInUse = errors.New("role is in use")

// ErrBuiltInRole is returned when an operation attempts to modify a built-in role.
var ErrBuiltInRole = errors.New("cannot modify built-in role")

// IsNotFound reports whether an error represents a "not found" condition,
// either from ErrNotFound or a wrapped pgx.ErrNoRows.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, pgx.ErrNoRows)
}
