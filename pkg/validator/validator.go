package validator

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

var v = validator.New()

// Validate checks a struct against its `validate` tags.
// Returns a human-readable error string or nil if valid.
//
// Usage:
//
//	type CreateUserRequest struct {
//	    Email    string `json:"email"    validate:"required,email"`
//	    Password string `json:"password" validate:"required,min=8"`
//	    Phone    string `json:"phone"    validate:"required"`
//	}
//
//	if err := validator.Validate(req); err != nil {
//	    response.UnprocessableEntity(w, err.Error())
//	    return
//	}
func Validate(s interface{}) error {
	err := v.Struct(s)
	if err == nil {
		return nil
	}

	var errs []string
	for _, e := range err.(validator.ValidationErrors) {
		errs = append(errs, fieldError(e))
	}
	return fmt.Errorf("%s", strings.Join(errs, "; "))
}

// fieldError converts a single ValidationError into a readable message.
func fieldError(e validator.FieldError) string {
	field := strings.ToLower(e.Field())

	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, e.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, e.Param())
	case "uuid4":
		return fmt.Sprintf("%s must be a valid UUID", field)
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, e.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, e.Param())
	case "gte":
		return fmt.Sprintf("%s must be %s or greater", field, e.Param())
	default:
		return fmt.Sprintf("%s is invalid", field)
	}
}