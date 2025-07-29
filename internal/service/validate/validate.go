package validate

import (
	"errors"
)

func Luhn(number string) error {
	// Convert number in digits and save in slice in reverse order
	// It's ok to work with string as bytes here
	digits := make([]int, 0, len(number))
	for i := len(number) - 1; i >= 0; i-- {
		n := number[i]
		if n < '0' || n > '9' {
			return errors.New("number contains invalid characters")
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

	switch sum % 10 {
	case 0:
		return nil
	default:
		return errors.New("number is not valid according to Luhn algorithm")
	}
}
