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
