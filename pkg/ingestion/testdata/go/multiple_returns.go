package sample

import "errors"

// Divide divides two numbers and returns quotient and remainder.
func Divide(a, b int) (quotient, remainder int, err error) {
	if b == 0 {
		return 0, 0, errors.New("division by zero")
	}
	quotient = a / b
	remainder = a % b
	return
}

// ParseInt parses a string to an integer.
func ParseInt(s string) (int, error) {
	// Simplified implementation
	return 0, nil
}
