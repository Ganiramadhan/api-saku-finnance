package validator

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	pv "github.com/go-playground/validator/v10"
)

type FieldError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Message string `json:"message"`
}

type ValidationError struct {
	Errors []FieldError `json:"errors"`
}

func (v *ValidationError) Error() string {
	parts := make([]string, 0, len(v.Errors))
	for _, e := range v.Errors {
		parts = append(parts, e.Message)
	}
	return strings.Join(parts, "; ")
}

type Validator struct {
	v *pv.Validate
}

func New() *Validator {
	v := pv.New(pv.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "" || name == "-" {
			return fld.Name
		}
		return name
	})
	return &Validator{v: v}
}

func (v *Validator) Struct(s any) error {
	if err := v.v.Struct(s); err != nil {
		var invalid *pv.InvalidValidationError
		if errors.As(err, &invalid) {
			return err
		}
		var ve pv.ValidationErrors
		if errors.As(err, &ve) {
			out := &ValidationError{Errors: make([]FieldError, 0, len(ve))}
			for _, fe := range ve {
				out.Errors = append(out.Errors, FieldError{
					Field:   fe.Field(),
					Tag:     fe.Tag(),
					Message: humanize(fe),
				})
			}
			return out
		}
		return err
	}
	return nil
}

func AsValidation(err error) (*ValidationError, bool) {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve, true
	}
	return nil, false
}

func humanize(fe pv.FieldError) string {
	field := fe.Field()
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, fe.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, fe.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, fe.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, fe.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", field, fe.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s", field, fe.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of [%s]", field, fe.Param())
	case "uuid":
		return fmt.Sprintf("%s must be a valid UUID", field)
	default:
		return fmt.Sprintf("%s failed on %s validation", field, fe.Tag())
	}
}
