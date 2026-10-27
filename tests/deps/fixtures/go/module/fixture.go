package fixture

import "github.com/google/uuid"

// New returns a new identifier.
func New() string { return uuid.NewString() }
