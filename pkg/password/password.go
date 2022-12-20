package password

import "strings"

// Password is an object that tries to make it hard (or explicit) to leak secrets
type Password struct {
	secret string
}

// New create a new password
func New(secret string) Password {
	return Password{secret: secret}
}

// String prevent leaking secrets by accident
func (p Password) String() string {
	return strings.Repeat("*", len(p.secret))
}

// Leak make it clear that we are leaking a secret
func (p Password) Leak() string {
	return p.secret
}

// Empty either the password is empty or not
func (p Password) Empty() bool {
	return len(p.secret) == 0
}
