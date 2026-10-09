package domain

import "regexp"

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// IDFormat describes the format ValidID accepts, for error messages.
const IDFormat = "must be 1 to 64 characters: letters, digits, underscore or hyphen"

// ValidID reports whether s is a well-formed user, item, reservation or
// request ID.
func ValidID(s string) bool {
	return idPattern.MatchString(s)
}
