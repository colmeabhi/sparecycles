package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
)

// sessionSecretSize is the length of a session secret in bytes (256 bits).
const sessionSecretSize = 32

// ErrBadSession means that a request has no session secret, or the wrong one.
var ErrBadSession = errors.New("missing or wrong session secret: register again")

// SessionHash is the SHA-256 hash of a session secret. The coordinator keeps
// only the hash, so a memory dump or a log line never shows the secret itself.
type SessionHash [sha256.Size]byte

// NewSession returns a new random session secret for a volunteer, and its hash.
// The coordinator sends the secret to the volunteer once, at registration, and
// keeps the hash. Gossip shares volunteer IDs with everyone, so an ID alone does
// not prove who sends a request; the secret does.
func NewSession() (secret []byte, hash SessionHash) {
	secret = make([]byte, sessionSecretSize)
	// Since Go 1.24, crypto/rand.Read never returns an error.
	_, _ = rand.Read(secret)
	return secret, sha256.Sum256(secret)
}

// Check returns nil if the secret matches the hash, else ErrBadSession.
// It compares in constant time, so the response time does not show how much
// of a guess was right.
func (h SessionHash) Check(secret []byte) error {
	if len(secret) != sessionSecretSize {
		return ErrBadSession
	}
	got := sha256.Sum256(secret)
	if subtle.ConstantTimeCompare(got[:], h[:]) != 1 {
		return ErrBadSession
	}
	return nil
}
