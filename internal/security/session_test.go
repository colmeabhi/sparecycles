package security

import (
	"bytes"
	"errors"
	"testing"
)

func TestNewSessionIsRandomAndFullLength(t *testing.T) {
	a, _ := NewSession()
	b, _ := NewSession()
	if len(a) != sessionSecretSize || len(b) != sessionSecretSize {
		t.Fatalf("secret lengths = %d, %d, want %d", len(a), len(b), sessionSecretSize)
	}
	if bytes.Equal(a, b) {
		t.Fatal("two sessions got the same secret")
	}
}

func TestSessionCheck(t *testing.T) {
	secret, hash := NewSession()
	if err := hash.Check(secret); err != nil {
		t.Fatalf("right secret: %v", err)
	}

	changed := bytes.Clone(secret)
	changed[0] ^= 1
	other, _ := NewSession()
	cases := map[string][]byte{
		"changed byte":   changed,
		"other session":  other,
		"empty":          nil,
		"too short":      secret[:sessionSecretSize-1],
		"too long":       append(bytes.Clone(secret), 0),
		"the hash bytes": hash[:],
	}
	for name, s := range cases {
		if err := hash.Check(s); !errors.Is(err, ErrBadSession) {
			t.Errorf("%s: err = %v, want ErrBadSession", name, err)
		}
	}
}

func TestZeroSessionHashRejectsEverything(t *testing.T) {
	var zero SessionHash
	secret, _ := NewSession()
	if err := zero.Check(secret); !errors.Is(err, ErrBadSession) {
		t.Fatalf("err = %v, want ErrBadSession", err)
	}
}
