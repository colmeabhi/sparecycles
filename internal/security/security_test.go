package security

import (
	"bytes"
	"errors"
	"testing"
)

func newKeyPair(t *testing.T) (*Ed25519Signer, *Ed25519Verifier) {
	t.Helper()
	seed, pub, err := GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewEd25519Signer(seed)
	if err != nil {
		t.Fatal(err)
	}
	v, err := NewEd25519Verifier(pub)
	if err != nil {
		t.Fatal(err)
	}
	if s.PublicKeyBase64() != pub {
		t.Fatal("the signer's public key must match the generated public key")
	}
	return s, v
}

func TestSignatureChecks(t *testing.T) {
	signer, verifier := newKeyPair(t)
	_, otherVerifier := newKeyPair(t)
	msg := TaskMessage("job-1-0", "fractal", 1_000, []byte("payload"))
	sig := signer.Sign(msg)

	if err := verifier.Verify(msg, sig); err != nil {
		t.Fatalf("valid signature: %v", err)
	}
	cases := map[string]struct {
		msg []byte
		sig []byte
		v   Verifier
	}{
		"changed payload":  {TaskMessage("job-1-0", "fractal", 1_000, []byte("payloaX")), sig, verifier},
		"changed unit":     {TaskMessage("job-1-1", "fractal", 1_000, []byte("payload")), sig, verifier},
		"changed workload": {TaskMessage("job-1-0", "montecarlo", 1_000, []byte("payload")), sig, verifier},
		"changed expiry":   {TaskMessage("job-1-0", "fractal", 2_000, []byte("payload")), sig, verifier},
		"wrong key":        {msg, sig, otherVerifier},
	}
	for name, c := range cases {
		if err := c.v.Verify(c.msg, c.sig); !errors.Is(err, ErrBadSignature) {
			t.Errorf("%s: err = %v, want ErrBadSignature", name, err)
		}
	}
	if err := verifier.Verify(msg, nil); !errors.Is(err, ErrMissingSignature) {
		t.Errorf("missing signature: err = %v, want ErrMissingSignature", err)
	}
}

// Without length prefixes, ("ab", "c") and ("a", "bc") would give the same bytes.
func TestTaskMessageIsUnambiguous(t *testing.T) {
	a := TaskMessage("ab", "c", 1, nil)
	b := TaskMessage("a", "bc", 1, nil)
	if bytes.Equal(a, b) {
		t.Fatal("two different tasks give the same message")
	}
}

func TestBadKeys(t *testing.T) {
	for _, k := range []string{"not base64!", "c2hvcnQ="} {
		if _, err := NewEd25519Signer(k); err == nil {
			t.Errorf("signer key %q: expected an error", k)
		}
		if _, err := NewEd25519Verifier(k); err == nil {
			t.Errorf("public key %q: expected an error", k)
		}
	}
}

func TestTokenList(t *testing.T) {
	l, err := NewTokenList([]string{"alice=secret-a", "secret-b"})
	if err != nil {
		t.Fatal(err)
	}
	if name, err := l.Authenticate("secret-a"); err != nil || name != "alice" {
		t.Fatalf("named token: name=%q err=%v", name, err)
	}
	if name, err := l.Authenticate("secret-b"); err != nil || name != "token-2" {
		t.Fatalf("unnamed token: name=%q err=%v", name, err)
	}
	if _, err := l.Authenticate("nope"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("unknown token: err = %v", err)
	}
	if _, err := l.Authenticate(""); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("empty token: err = %v", err)
	}
}

func TestBadTokenLists(t *testing.T) {
	cases := map[string][]string{
		"empty list":      {},
		"empty token":     {"alice="},
		"empty name":      {"=secret"},
		"repeated token":  {"a=x", "b=x"},
		"only whitespace": {"  "},
	}
	for name, items := range cases {
		if _, err := NewTokenList(items); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestSharedToken(t *testing.T) {
	if label, err := (SharedToken{}).Authenticate("anything"); err != nil || label != "open" {
		t.Fatalf("open: label=%q err=%v", label, err)
	}
	s := SharedToken{Token: "t"}
	if _, err := s.Authenticate("t"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate("x"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("wrong token: err = %v", err)
	}
}

func TestNoSignAcceptsEverything(t *testing.T) {
	if sig := (NoSign{}).Sign([]byte("x")); sig != nil {
		t.Fatal("NoSign must not sign")
	}
	if err := (NoSign{}).Verify([]byte("x"), nil); err != nil {
		t.Fatal(err)
	}
}
