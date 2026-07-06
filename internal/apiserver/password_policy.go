package apiserver

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

var weakPasswords = map[string]struct{}{
	"admin123456":     {},
	"changeme123":     {},
	"changeme123!":    {},
	"change-me-first": {},
	"letmein123456":   {},
	"password123":     {},
	"password1234":    {},
	"password123!":    {},
	"qwerty123456":    {},
}

func ValidatePassword(password string) error {
	length := utf8.RuneCountInString(password)
	if length < 12 || length > 128 {
		return errors.New("password must be 12-128 characters")
	}
	if _, weak := weakPasswords[strings.ToLower(strings.TrimSpace(password))]; weak {
		return errors.New("password is too common")
	}

	classes := 0
	hasLower := false
	hasUpper := false
	hasDigit := false
	hasSymbol := false
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		case !unicode.IsSpace(r):
			hasSymbol = true
		}
	}
	for _, ok := range []bool{hasLower, hasUpper, hasDigit, hasSymbol} {
		if ok {
			classes++
		}
	}
	if classes < 3 {
		return errors.New("password must include at least 3 of lowercase, uppercase, number, and symbol")
	}
	return nil
}
