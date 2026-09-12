package ghostty

import (
	"context"
	"fmt"
)

// Validator is the seam for invoking Ghostty's authoritative config checks.
type Validator interface {
	Validate(context.Context, string) error
}

// UnavailableValidator makes the missing-binary state explicit to callers.
type UnavailableValidator struct{}

func (UnavailableValidator) Validate(context.Context, string) error {
	return fmt.Errorf("Ghostty is not installed or could not be located")
}
