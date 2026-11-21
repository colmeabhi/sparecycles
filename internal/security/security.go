// Package security authenticates volunteers and signs tasks.
//
// Owner: Kushagra (security).
// Coordinators check a volunteer token at registration and sign each task they
// hand out. Volunteers check the signature before they run a task, so they run
// only tasks that came from a coordinator with the team's signing key.
package security

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
)

// ErrUnauthenticated means that the token is not valid.
var ErrUnauthenticated = errors.New("invalid volunteer token")

// Authenticator checks volunteer tokens.
type Authenticator interface {
	// Authenticate returns a label for logs, for example the token's name, or an error.
	Authenticate(token string) (label string, err error)
}

// Signer signs task messages. The coordinator uses it.
type Signer interface {
	Sign(msg []byte) []byte
}

// Verifier checks task signatures. The volunteer uses it.
type Verifier interface {
	Verify(msg, signature []byte) error
}

// SharedToken accepts one token for all volunteers.
// If Token is empty, it accepts every volunteer.
type SharedToken struct {
	Token string
}

// Authenticate compares the token in constant time.
func (s SharedToken) Authenticate(token string) (string, error) {
	if s.Token == "" {
		return "open", nil
	}
	if subtle.ConstantTimeCompare([]byte(s.Token), []byte(token)) != 1 {
		return "", ErrUnauthenticated
	}
	return "shared", nil
}

// TokenList accepts a fixed list of per-volunteer tokens.
type TokenList struct {
	entries []tokenEntry
}

type tokenEntry struct {
	name  string
	token []byte
}

// NewTokenList parses entries of the form "name=token" or "token".
// A token without a name gets the name "token-N", where N starts at 1.
func NewTokenList(items []string) (*TokenList, error) {
	l := &TokenList{}
	seen := map[string]bool{}
	for i, item := range items {
		name, token, hasName := strings.Cut(strings.TrimSpace(item), "=")
		if !hasName {
			name, token = fmt.Sprintf("token-%d", i+1), name
		}
		name, token = strings.TrimSpace(name), strings.TrimSpace(token)
		if token == "" || name == "" {
			return nil, fmt.Errorf("token entry %d is empty", i+1)
		}
		if seen[token] {
			return nil, fmt.Errorf("token entry %d (%s) repeats an earlier token", i+1, name)
		}
		seen[token] = true
		l.entries = append(l.entries, tokenEntry{name: name, token: []byte(token)})
	}
	if len(l.entries) == 0 {
		return nil, errors.New("the token list is empty")
	}
	return l, nil
}

// Authenticate returns the name of the matching token. It compares every entry
// in constant time, so the response time does not show which entry was close.
func (l *TokenList) Authenticate(token string) (string, error) {
	match := ""
	for _, e := range l.entries {
		if subtle.ConstantTimeCompare(e.token, []byte(token)) == 1 {
			match = e.name
		}
	}
	if match == "" {
		return "", ErrUnauthenticated
	}
	return match, nil
}

// NoSign does not sign or check anything. It is used when no key is set.
type NoSign struct{}

// Sign returns nil.
func (NoSign) Sign([]byte) []byte { return nil }

// Verify accepts every message.
func (NoSign) Verify([]byte, []byte) error { return nil }
