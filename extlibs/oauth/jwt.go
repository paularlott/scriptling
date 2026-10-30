package oauth

import (
	"bytes"
	"crypto"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// The JSON Web Token and Key handling verify() needs, on Go's standard
// crypto only: JWS compact serialisation, the asymmetric algorithms identity
// providers sign access tokens with, and the public keys of a JWK set. Symmetric
// (HS*) and "none" are never accepted: a resource server holds no shared secret.

// jwtParts is a parsed, not yet verified, compact JWT.
type jwtParts struct {
	header       map[string]any
	claims       map[string]any
	signingInput []byte // header.payload, as signed
	signature    []byte
}

// parseJWT splits and decodes a compact JWT. Nothing in it is trusted yet.
func parseJWT(token string) (*jwtParts, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("not a JWT")
	}
	header, err := decodeSegment(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid header: %w", err)
	}
	claims, err := decodeSegment(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid claims: %w", err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("invalid signature encoding")
	}
	return &jwtParts{header: header, claims: claims, signingInput: []byte(parts[0] + "." + parts[1]), signature: sig}, nil
}

// decodeSegment decodes a base64url JSON object, keeping numbers exact.
func decodeSegment(segment string) (map[string]any, error) {
	raw, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return nil, errors.New("invalid base64url")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var out map[string]any
	if err := dec.Decode(&out); err != nil || out == nil {
		return nil, errors.New("not a JSON object")
	}
	return out, nil
}

// jwk is one key of a JWK set (RFC 7517), the fields verify() reads.
type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// publicKey is a parsed signing key with the algorithm family it serves.
type publicKey struct {
	kid string
	alg string // the key's own alg, empty when it names none
	key crypto.PublicKey
}

// parseJWKS parses a JWK set, keeping the signing keys it understands and
// skipping the rest (encryption keys, unknown types), as a resource server
// must for sets that hold more than it needs.
func parseJWKS(raw []byte) ([]publicKey, error) {
	var set struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.Unmarshal(raw, &set); err != nil {
		return nil, errors.New("the JWK set is not valid JSON")
	}
	var keys []publicKey
	for _, k := range set.Keys {
		if k.Use == "enc" {
			continue
		}
		pub, err := k.publicKey()
		if err != nil {
			continue
		}
		keys = append(keys, publicKey{kid: k.Kid, alg: k.Alg, key: pub})
	}
	if len(keys) == 0 {
		return nil, errors.New("the JWK set has no usable signing keys")
	}
	return keys, nil
}

func (k jwk) publicKey() (crypto.PublicKey, error) {
	switch k.Kty {
	case "EC":
		return ecKey(k.Crv, k.X, k.Y)
	case "RSA":
		n, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			return nil, err
		}
		e, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil || len(e) == 0 || len(e) > 4 {
			return nil, errors.New("invalid RSA exponent")
		}
		pub := &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
		if pub.N.BitLen() < 2048 {
			return nil, errors.New("RSA key shorter than 2048 bits")
		}
		return pub, nil
	case "OKP":
		if k.Crv != "Ed25519" {
			return nil, errors.New("unsupported OKP curve")
		}
		x, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil || len(x) != ed25519.PublicKeySize {
			return nil, errors.New("invalid Ed25519 key")
		}
		return ed25519.PublicKey(x), nil
	}
	return nil, errors.New("unsupported key type")
}

// ecKey builds an ECDSA public key, checking the point lies on the curve.
func ecKey(crv, xs, ys string) (*ecdsa.PublicKey, error) {
	var curve elliptic.Curve
	var ecdhCurve ecdh.Curve
	switch crv {
	case "P-256":
		curve, ecdhCurve = elliptic.P256(), ecdh.P256()
	case "P-384":
		curve, ecdhCurve = elliptic.P384(), ecdh.P384()
	case "P-521":
		curve, ecdhCurve = elliptic.P521(), ecdh.P521()
	default:
		return nil, errors.New("unsupported EC curve")
	}
	size := (curve.Params().BitSize + 7) / 8
	x, errX := base64.RawURLEncoding.DecodeString(xs)
	y, errY := base64.RawURLEncoding.DecodeString(ys)
	if errX != nil || errY != nil || len(x) != size || len(y) != size {
		return nil, errors.New("invalid EC key coordinates")
	}
	// ecdh refuses points off the curve and the point at infinity.
	if _, err := ecdhCurve.NewPublicKey(append(append([]byte{4}, x...), y...)); err != nil {
		return nil, errors.New("EC key is not on its curve")
	}
	return &ecdsa.PublicKey{Curve: curve, X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}, nil
}

// errUnknownKey is a token signed with a key the set does not hold (yet):
// the caller may fetch the set again in case the IdP rotated its keys.
var errUnknownKey = errors.New("no key of the IdP's JWK set matches the token")

// verifySignature checks the token's signature with the key it names (by
// kid, or any key that fits the algorithm when it names none).
func verifySignature(t *jwtParts, keys []publicKey) error {
	// RFC 7515 §4.1.11: extensions marked critical must be understood; none are.
	if _, ok := t.header["crit"]; ok {
		return errors.New("it has critical header extensions this does not support")
	}
	alg, _ := t.header["alg"].(string)
	if !supportedAlg(alg) {
		return fmt.Errorf("signature algorithm %q is not accepted", alg)
	}
	kid, _ := t.header["kid"].(string)
	tried := false
	for _, k := range keys {
		if kid != "" && k.kid != kid {
			continue
		}
		if k.alg != "" && k.alg != alg {
			continue
		}
		if !keyFitsAlg(k.key, alg) {
			continue
		}
		tried = true
		if checkSignature(k.key, alg, t.signingInput, t.signature) {
			return nil
		}
	}
	if !tried {
		return errUnknownKey
	}
	return errors.New("the signature does not verify")
}

func supportedAlg(alg string) bool {
	switch alg {
	case "ES256", "ES384", "ES512", "RS256", "RS384", "RS512", "PS256", "PS384", "PS512", "EdDSA":
		return true
	}
	return false
}

func keyFitsAlg(key crypto.PublicKey, alg string) bool {
	switch k := key.(type) {
	case *ecdsa.PublicKey:
		bits := map[string]int{"ES256": 256, "ES384": 384, "ES512": 521}[alg]
		return bits != 0 && k.Curve.Params().BitSize == bits
	case *rsa.PublicKey:
		return strings.HasPrefix(alg, "RS") || strings.HasPrefix(alg, "PS")
	case ed25519.PublicKey:
		return alg == "EdDSA"
	}
	return false
}

func checkSignature(key crypto.PublicKey, alg string, input, sig []byte) bool {
	if alg == "EdDSA" {
		return ed25519.Verify(key.(ed25519.PublicKey), input, sig)
	}
	hash := map[string]crypto.Hash{"256": crypto.SHA256, "384": crypto.SHA384, "512": crypto.SHA512}[alg[2:]]
	var digest []byte
	switch hash {
	case crypto.SHA256:
		d := sha256.Sum256(input)
		digest = d[:]
	case crypto.SHA384:
		d := sha512.Sum384(input)
		digest = d[:]
	case crypto.SHA512:
		d := sha512.Sum512(input)
		digest = d[:]
	default:
		return false
	}
	switch k := key.(type) {
	case *ecdsa.PublicKey:
		size := (k.Curve.Params().BitSize + 7) / 8
		if len(sig) != 2*size {
			return false
		}
		r := new(big.Int).SetBytes(sig[:size])
		s := new(big.Int).SetBytes(sig[size:])
		return ecdsa.Verify(k, digest, r, s)
	case *rsa.PublicKey:
		if strings.HasPrefix(alg, "PS") {
			return rsa.VerifyPSS(k, hash, digest, sig, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash}) == nil
		}
		return rsa.VerifyPKCS1v15(k, hash, digest, sig) == nil
	}
	return false
}
