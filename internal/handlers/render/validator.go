package render

import (
	"strings"

	"github.com/go-playground/validator/v10"
	"reflect"
)

func configureValidator(validate *validator.Validate) {
	_ = validate.RegisterValidation("luhn", validateLuhnAlgorithm)
	validate.RegisterTagNameFunc(useJSONTagNames)
}

func useJSONTagNames(fld reflect.StructField) string {
	name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
	// skip if tag key says it should be ignored
	if name == "-" {
		return ""
	}
	return name
}

func validateLuhnAlgorithm(fl validator.FieldLevel) bool {
	number := fl.Field().String()

	// Convert number in digits and save in slice in reverse order
	// It's ok to work with string as bytes here
	digits := make([]int, 0, len(number))
	for i := len(number) - 1; i >= 0; i-- {
		n := number[i]
		if n < '0' || n > '9' {
			return false
		}
		digits = append(digits, int(n-'0'))
	}

	sum := 0
	for i, digit := range digits {
		position := i + 1
		if position%2 == 0 {
			digit *= 2
			if digit > 9 {
				digit = (digit % 10) + 1
			}
		}

		sum += digit
	}

	return sum%10 == 0
}
