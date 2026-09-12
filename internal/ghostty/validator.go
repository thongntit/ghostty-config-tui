package ghostty

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

var ErrValidatorUnavailable = errors.New("Ghostty validator unavailable")

// Validator is the seam for invoking Ghostty's authoritative config checks.
type Validator interface {
	Validate(context.Context, string) error
}

// UnavailableValidator makes the missing-binary state explicit to callers.
type UnavailableValidator struct{}

func (UnavailableValidator) Validate(context.Context, string) error {
	return fmt.Errorf("Ghostty is not installed or could not be located")
}

// ValidateConfig invokes Ghostty's own parser against an explicit candidate
// file. The caller decides when a candidate is safe to place in a temporary
// location; this function never opens a shell and never writes user files.
func ValidateConfig(ctx context.Context, binary, path string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(binary) == "" {
		return fmt.Errorf("Ghostty executable is not configured")
	}
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("Ghostty config path is empty")
	}
	validationContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := exec.CommandContext(validationContext, binary, "+validate-config", "--config-file", path)
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if strings.Contains(message, "SentryInitFailed") {
			return fmt.Errorf("%w: %s", ErrValidatorUnavailable, message)
		}
		if message == "" {
			return fmt.Errorf("Ghostty rejected the candidate: %w", err)
		}
		return fmt.Errorf("Ghostty rejected the candidate: %s", message)
	}
	return nil
}
