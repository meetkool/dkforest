package password

import "strings"

// Password is an object that hides the actual secret value and provides methods to check its properties
type Password struct {
	secret string
}

// New creates a new Password object with the given secret value
func New(secret string) Password {
	return Password{secret: secret}
}

// Mask returns a masked version of the secret value
func (p Password) Mask() string {
	return strings.Repeat("*", len(p.secret))
}

// Reveal makes it clear that we are revealing the secret value
func (p Password) Reveal() string {
	return p.secret
}

// IsEmpty checks if the Password object has an empty secret value
func (p Password) IsEmpty() bool {
	return len(p.secret) == 0
}

// Validate checks if the secret value meets certain criteria (e.g. length, complexity)
func (p Password) Validate() error {
	// Implement password validation logic here
	if len(p.secret) < 8 {
		return ErrPasswordTooShort
	}
	// Add more validation checks as needed
	return nil
}

// ErrPasswordTooShort is an example error that can be returned by Validate method
var ErrPasswordTooShort = errors.New("password is too short")
