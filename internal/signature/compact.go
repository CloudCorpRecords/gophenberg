// SPDX-License-Identifier: Apache-2.0

package signature

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"errors"
	"math/big"
)

// CompactSize is the length of a compact P-256 signature, r then s, each in 32 bytes.
const CompactSize = 64

// errNotP256 refuses a compact signature key on a curve other than P-256.
var errNotP256 = errors.New("signature: refused an ECDSA key on a curve other than P-256")

// asn1Signature is the DER form of an ECDSA signature.
type asn1Signature struct {
	R, S *big.Int
}

// SignCompact returns the low s compact P-256 signature of the SHA-256 of message, in base64url without padding.
func SignCompact(key *ecdsa.PrivateKey, message []byte) (string, error) {
	if key.Curve != elliptic.P256() {
		return "", errNotP256
	}
	digest := sha256.Sum256(message)
	der, _ := ecdsa.SignASN1(rand.Reader, key, digest[:])
	var parsed asn1Signature
	_, _ = asn1.Unmarshal(der, &parsed)
	if parsed.S.Cmp(halfOrder()) > 0 {
		parsed.S.Sub(elliptic.P256().Params().N, parsed.S)
	}
	raw := make([]byte, CompactSize)
	parsed.R.FillBytes(raw[:CompactSize/2])
	parsed.S.FillBytes(raw[CompactSize/2:])
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// VerifyCompact reports whether a compact signature of the SHA-256 of message holds for key, refusing a high s.
func VerifyCompact(key *ecdsa.PublicKey, message []byte, signature string) error {
	if key.Curve != elliptic.P256() {
		return errNotP256
	}
	raw, err := decodeSignature(signature, CompactSize)
	if err != nil {
		return err
	}
	r, s := new(big.Int).SetBytes(raw[:CompactSize/2]), new(big.Int).SetBytes(raw[CompactSize/2:])
	if s.Cmp(halfOrder()) > 0 {
		return errors.New("signature: refused a P-256 signature with a high s, which has a second encoding")
	}
	digest := sha256.Sum256(message)
	if !ecdsa.Verify(key, digest[:], r, s) {
		return ErrMismatch
	}
	return nil
}

// halfOrder returns half the order of the P-256 group, the largest low s.
func halfOrder() *big.Int {
	return new(big.Int).Rsh(elliptic.P256().Params().N, 1)
}
