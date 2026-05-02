package validator

import (
	"strings"
	"testing"
)

type sample struct {
	Name  string `json:"name" validate:"required,min=2"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"gte=0,lte=120"`
}

func TestValidator_Struct_OK(t *testing.T) {
	v := New()
	if err := v.Struct(sample{Name: "John", Email: "john@example.com", Age: 30}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidator_Struct_FieldErrors(t *testing.T) {
	v := New()
	err := v.Struct(sample{Name: "", Email: "not-an-email", Age: 200})
	if err == nil {
		t.Fatal("expected error")
	}
	ve, ok := AsValidation(err)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if len(ve.Errors) < 3 {
		t.Fatalf("expected at least 3 field errors, got %d", len(ve.Errors))
	}

	// Field names should come from the json tag.
	fields := map[string]bool{}
	for _, fe := range ve.Errors {
		fields[fe.Field] = true
	}
	for _, want := range []string{"name", "email", "age"} {
		if !fields[want] {
			t.Errorf("missing field error for %q (got %v)", want, fields)
		}
	}

	// Aggregated message should be non-empty + readable.
	if msg := ve.Error(); msg == "" || !strings.Contains(msg, "email") {
		t.Errorf("unexpected aggregated error message: %q", msg)
	}
}

func TestValidator_Humanize_KnownTags(t *testing.T) {
	v := New()
	type s struct {
		Role string `json:"role" validate:"oneof=user admin"`
	}
	err := v.Struct(s{Role: "guest"})
	ve, ok := AsValidation(err)
	if !ok {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if !strings.Contains(ve.Errors[0].Message, "must be one of") {
		t.Errorf("expected human-friendly oneof message, got %q", ve.Errors[0].Message)
	}
}
