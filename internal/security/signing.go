package security

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
)

var (
	// ErrMissingSignature means that a task has no signature but one is required.
	ErrMissingSignature = errors.New("task has no signature")
	// ErrBadSignature means that the signature does not match the task.
	ErrBadSignature = errors.New("task signature does not match")
)

// taskDomain separates task signatures from any other use of the same key.
const taskDomain = "sparecycles/task/v1"

// TaskMessage returns the bytes that the coordinator signs for one task.
//
// It binds the payload to its unit, workload and lease expiry, so a signed task
// can not be replayed for another unit or after its lease. Each field has a
// length prefix, so two different tasks never give the same bytes.
func TaskMessage(unitID, workload string, leaseExpiryUnixMs int64, payload []byte) []byte {
	msg := make([]byte, 0, len(taskDomain)+len(unitID)+len(workload)+len(payload)+40)
	msg = appendField(msg, []byte(taskDomain))
	msg = appendField(msg, []byte(unitID))
	msg = appendField(msg, []byte(workload))
	msg = binary.BigEndian.AppendUint64(msg, uint64(leaseExpiryUnixMs))
	msg = appendField(msg, payload)
	return msg
}

func appendField(dst, field []byte) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(field)))
	return append(dst, field...)
}

// Ed25519Signer signs tasks with a private key. Coordinators use it.
type Ed25519Signer struct {
	private ed25519.PrivateKey
}

// NewEd25519Signer reads a base64 private key seed (32 bytes), as made by GenerateKeys.
func NewEd25519Signer(seedBase64 string) (*Ed25519Signer, error) {
	seed, err := base64.StdEncoding.DecodeString(seedBase64)
	if err != nil {
		return nil, fmt.Errorf("signing key is not base64: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("signing key has %d bytes, want %d", len(seed), ed25519.SeedSize)
	}
	return &Ed25519Signer{private: ed25519.NewKeyFromSeed(seed)}, nil
}

// Sign returns the Ed25519 signature of msg.
func (s *Ed25519Signer) Sign(msg []byte) []byte {
	return ed25519.Sign(s.private, msg)
}

// PublicKeyBase64 returns the matching public key, for volunteers.
func (s *Ed25519Signer) PublicKeyBase64() string {
	return base64.StdEncoding.EncodeToString(s.private.Public().(ed25519.PublicKey))
}

// Ed25519Verifier checks task signatures with a public key. Volunteers use it.
type Ed25519Verifier struct {
	public ed25519.PublicKey
}

// NewEd25519Verifier reads a base64 public key (32 bytes).
func NewEd25519Verifier(publicBase64 string) (*Ed25519Verifier, error) {
	pub, err := base64.StdEncoding.DecodeString(publicBase64)
	if err != nil {
		return nil, fmt.Errorf("signing public key is not base64: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("signing public key has %d bytes, want %d", len(pub), ed25519.PublicKeySize)
	}
	return &Ed25519Verifier{public: ed25519.PublicKey(pub)}, nil
}

// Verify checks that signature is a valid signature of msg.
func (v *Ed25519Verifier) Verify(msg, signature []byte) error {
	if len(signature) == 0 {
		return ErrMissingSignature
	}
	if !ed25519.Verify(v.public, msg, signature) {
		return ErrBadSignature
	}
	return nil
}

// GenerateKeys returns a new key pair: the private seed and the public key, both in base64.
func GenerateKeys() (seedBase64, publicBase64 string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(priv.Seed()), base64.StdEncoding.EncodeToString(pub), nil
}
