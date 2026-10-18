// SPDX-License-Identifier: Apache-2.0

package signature_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/signature"
)

// rfc6979 is the P-256 key and SHA-256 signature of "sample" that RFC 6979 A.2.5 publishes, s as published.
var rfc6979 = struct {
	d, r, s string
}{
	d: "C9AFA9D845BA75166B5C215767B1D6934E50C3DB36E89B127B8A622B120F6721",
	r: "EFD48B2AACB6A8FD1140DD9CD45E81D69D2C877B56AAF991C34D0EA84EAF3716",
	s: "F7CB1C942D657C41D436C7A1B6E29F65F3E900DBB9AFF4064DC4AB2F843ACDA8",
}

// newP256 returns a fresh P-256 signing key.
func newP256(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("making a P-256 key: %v", err)
	}
	return key
}

// hexInt returns the number a hexadecimal text names.
func hexInt(t *testing.T, text string) *big.Int {
	t.Helper()
	n, ok := new(big.Int).SetString(text, 16)
	if !ok {
		t.Fatalf("reading %q as hexadecimal", text)
	}
	return n
}

// compactOf returns r and s written as a compact signature in base64url without padding.
func compactOf(r, s *big.Int) string {
	raw := make([]byte, signature.CompactSize)
	r.FillBytes(raw[:signature.CompactSize/2])
	s.FillBytes(raw[signature.CompactSize/2:])
	return base64.RawURLEncoding.EncodeToString(raw)
}

// rfc6979Key returns the public key of the RFC 6979 P-256 vector.
func rfc6979Key(t *testing.T) *ecdsa.PublicKey {
	t.Helper()
	d, err := hex.DecodeString(rfc6979.d)
	if err != nil {
		t.Fatalf("reading the vector's private key: %v", err)
	}
	key, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), d)
	if err != nil {
		t.Fatalf("parsing the vector's private key: %v", err)
	}
	return &key.PublicKey
}

func TestCompactSignaturesVerifyUnderTheKeyThatMadeThem(t *testing.T) {
	t.Parallel()

	key := newP256(t)
	message := []byte(`{"schema":1,"serial":1}`)

	signed, err := signature.SignCompact(key, message)

	if err != nil {
		t.Fatalf("SignCompact() error = %v, want nil", err)
	}
	if err := signature.VerifyCompact(&key.PublicKey, message, signed); err != nil {
		t.Errorf("VerifyCompact() = %v on the signature SignCompact made, want nil", err)
	}
}

func TestSignCompactAlwaysWritesTheLowS(t *testing.T) {
	t.Parallel()

	key := newP256(t)
	half := new(big.Int).Rsh(elliptic.P256().Params().N, 1)
	for i := range 64 {
		signed, err := signature.SignCompact(key, []byte{byte(i)})
		if err != nil {
			t.Fatalf("SignCompact() error = %v, want nil", err)
		}
		raw, _ := base64.RawURLEncoding.DecodeString(signed)
		if s := new(big.Int).SetBytes(raw[signature.CompactSize/2:]); s.Cmp(half) > 0 {
			t.Fatalf("SignCompact() wrote s = %x, above half the curve order", s)
		}
	}
}

func TestVerifyCompactTakesThePublishedVectorOnlyInItsLowForm(t *testing.T) {
	t.Parallel()

	key, message := rfc6979Key(t), []byte("sample")
	r, s := hexInt(t, rfc6979.r), hexInt(t, rfc6979.s)
	digest := sha256.Sum256(message)
	if !ecdsa.Verify(key, digest[:], r, s) {
		t.Fatal("the RFC 6979 vector does not verify as published, so the vector is wrong")
	}
	low := new(big.Int).Sub(elliptic.P256().Params().N, s)

	if err := signature.VerifyCompact(key, message, compactOf(r, low)); err != nil {
		t.Errorf("VerifyCompact() = %v on the vector's low form, want nil", err)
	}
	if err := signature.VerifyCompact(key, message, compactOf(r, s)); err == nil ||
		!strings.Contains(err.Error(), "high s") {
		t.Errorf("VerifyCompact() = %v on the vector as published, want its high s refused", err)
	}
}

func TestVerifyCompactRefusesAChangedMessage(t *testing.T) {
	t.Parallel()

	key := newP256(t)
	signed, err := signature.SignCompact(key, []byte("keys"))
	if err != nil {
		t.Fatalf("SignCompact() error = %v, want nil", err)
	}

	if err := signature.VerifyCompact(&key.PublicKey, []byte("keys!"), signed); !errors.Is(err, signature.ErrMismatch) {
		t.Errorf("VerifyCompact() = %v on a changed message, want ErrMismatch", err)
	}
	if err := signature.VerifyCompact(&newP256(t).PublicKey, []byte("keys"), signed); !errors.Is(err,
		signature.ErrMismatch) {
		t.Errorf("VerifyCompact() = %v under another key, want ErrMismatch", err)
	}
}

func TestVerifyCompactRefusesEveryOtherEncoding(t *testing.T) {
	t.Parallel()

	key := newP256(t)
	signed, err := signature.SignCompact(key, []byte("keys"))
	if err != nil {
		t.Fatalf("SignCompact() error = %v, want nil", err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(signed)

	for name, text := range map[string]string{
		"standard base64": base64.StdEncoding.EncodeToString(raw),
		"a broken line":   signed[:20] + "\n" + signed[20:],
		"63 bytes":        base64.RawURLEncoding.EncodeToString(raw[:63]),
		"65 bytes":        base64.RawURLEncoding.EncodeToString(append(raw, 0)),
		"not base64":      "!" + signed[1:],
		"a zero s":        compactOf(new(big.Int).SetBytes(raw[:32]), new(big.Int)),
	} {
		if err := signature.VerifyCompact(&key.PublicKey, []byte("keys"), text); err == nil {
			t.Errorf("VerifyCompact() on %s = nil, want it refused", name)
		}
	}
}

func TestVerifyCompactRefusesAKeyOnAnotherCurve(t *testing.T) {
	t.Parallel()

	other, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("making a P-384 key: %v", err)
	}
	signed, err := signature.SignCompact(newP256(t), []byte("keys"))
	if err != nil {
		t.Fatalf("SignCompact() error = %v, want nil", err)
	}

	if err := signature.VerifyCompact(&other.PublicKey, []byte("keys"), signed); err == nil {
		t.Error("VerifyCompact() under a P-384 key = nil, want it refused")
	}
	if _, err := signature.SignCompact(other, []byte("keys")); err == nil {
		t.Error("SignCompact() with a P-384 key = nil, want it refused")
	}
}
