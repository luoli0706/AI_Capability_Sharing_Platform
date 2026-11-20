package validator

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// CustomValidator adapts go-playground/validator/v10 for Echo.
type CustomValidator struct {
	validate *validator.Validate
}

// New creates a new CustomValidator instance.
func New() *CustomValidator {
	return &CustomValidator{
		validate: validator.New(),
	}
}

// Validate validates a struct and returns a readable error message on failure.
func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validate.Struct(i); err != nil {
		if errs, ok := err.(validator.ValidationErrors); ok {
			msgs := make([]string, 0, len(errs))
			for _, e := range errs {
				msgs = append(msgs, fmt.Sprintf("field '%s' failed on '%s' tag", e.Field(), e.Tag()))
			}
			return fmt.Errorf("%s", strings.Join(msgs, "; "))
		}
		return err
	}
	return nil
}
