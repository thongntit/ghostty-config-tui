package schema

import (
	"strings"
	"testing"
)

func TestNumberValidationRejectsNonFiniteAndAcceptsFiniteValues(t *testing.T) {
	option := Option{Key: "font-size", Kind: KindNumber, Edit: EditScalar}
	for _, value := range []string{"NaN", "+Inf", "-Inf", "not-a-number"} {
		if err := option.Validate(value); err == nil {
			t.Errorf("expected %q to be rejected", value)
		}
	}
	if err := option.Validate("14.5"); err != nil {
		t.Fatalf("finite number rejected: %v", err)
	}
}

func TestValidationReservesEmptyValueForReset(t *testing.T) {
	option := Option{Key: "theme", Kind: KindString, Edit: EditScalar}
	if err := option.Validate(""); err == nil || !strings.Contains(err.Error(), "reset") {
		t.Fatalf("unexpected empty-value validation: %v", err)
	}
}

func TestEnumValidationUsesDeclaredValues(t *testing.T) {
	option := Option{Key: "cursor-style", Kind: KindEnum, Edit: EditScalar, Values: []string{"block", "bar"}}
	if err := option.Validate("underline"); err == nil {
		t.Fatal("undeclared enum value accepted")
	}
	if err := option.Validate("bar"); err != nil {
		t.Fatalf("declared enum value rejected: %v", err)
	}
}

func TestMultipleValidationAcceptsNegatedAndBooleanForms(t *testing.T) {
	option := Option{Key: "shell-integration-features", Kind: KindString, Multiple: true, Values: []string{"cursor", "title"}}
	for _, value := range []string{"cursor,no-title", "true", "false", "title"} {
		if err := option.Validate(value); err != nil {
			t.Errorf("multiple value %q rejected: %v", value, err)
		}
	}
	for _, value := range []string{"cursor,", "cursor,unknown", "no-unknown"} {
		if err := option.Validate(value); err == nil {
			t.Errorf("invalid multiple value %q accepted", value)
		}
	}
}
