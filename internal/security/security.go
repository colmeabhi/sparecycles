// Package security authenticates volunteers and signs tasks.
//
// Owner: Kushagra (security).
// The base version checks one shared token and does not sign tasks.
// The real version gives each volunteer its own token and signs each task,
// so that a volunteer can reject a task that did not come from the leader.
package security

import (
	"crypto/subtle"
	"errors"
)

// ErrUnauthenticated means that the token is not valid.
var ErrUnauthenticated = errors.New("invalid volunteer token")

// Authenticator checks volunteer tokens.
type Authenticator interface {
	Authenticate(token string) error
}

// Signer signs task payloads and checks signatures.
type Signer interface {
	Sign(payload []byte) []byte
	Verify(payload, signature []byte) error
}

// SharedToken accepts one token for all volunteers.
// If Token is empty, it accepts every volunteer.
type SharedToken struct {
	Token string
}

// Authenticate compares the token in constant time.
func (s SharedToken) Authenticate(token string) error {
	if s.Token == "" {
		return nil
	}
	if subtle.ConstantTimeCompare([]byte(s.Token), []byte(token)) != 1 {
		return ErrUnauthenticated
	}
	return nil
}

// NoSign does not sign. It is the base stub.
type NoSign struct{}

// Sign returns nil.
func (NoSign) Sign([]byte) []byte { return nil }

// Verify accepts every payload.
func (NoSign) Verify([]byte, []byte) error { return nil }
