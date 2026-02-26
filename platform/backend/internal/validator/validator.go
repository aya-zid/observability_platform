// internal/validator/validator.go
package validator

import "github.com/go-playground/validator/v10"

var Validate *validator.Validate

func init() {
	Validate = validator.New()
}

// ValidateStruct validates any struct with validator tags
func ValidateStruct(s interface{}) error {
	return Validate.Struct(s)
}