package mercure

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"strings"
	"time"
)

// detailType is the type of a Mercure authorization detail (Mercure 1.0).
const detailType = "https://mercure.rocks/authorization-detail"

// signToken returns a JWT signed with an HMAC key (HS256, HS384 or HS512): a
// Mercure hub's publisher or subscriber key, as hubs are most often set up.
// typ is the header's typ: "JWT", or "at+jwt" for an access token.
func signToken(key []byte, algorithm, typ string, claims map[string]any) (string, error) {
	var h func() hash.Hash
	switch strings.ToUpper(algorithm) {
	case "HS256":
		h = sha256.New
	case "HS384":
		h = sha512.New384
	case "HS512":
		h = sha512.New
	default:
		return "", fmt.Errorf("algorithm must be HS256, HS384 or HS512, not %q", algorithm)
	}
	if len(key) == 0 {
		return "", fmt.Errorf("key is empty")
	}
	header, err := json.Marshal(map[string]string{"alg": strings.ToUpper(algorithm), "typ": typ})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("claims: %w", err)
	}
	enc := base64.RawURLEncoding
	signing := enc.EncodeToString(header) + "." + enc.EncodeToString(payload)
	mac := hmac.New(h, key)
	mac.Write([]byte(signing))
	return signing + "." + enc.EncodeToString(mac.Sum(nil)), nil
}

// mercureClaims are the claims of a draft-07 Mercure token: the "mercure"
// claim with its publish and subscribe selectors, and exp when expires > 0.
func mercureClaims(publish, subscribe []string, payload any, expires time.Duration, now time.Time) map[string]any {
	m := map[string]any{}
	if publish != nil {
		m["publish"] = publish
	}
	if subscribe != nil {
		m["subscribe"] = subscribe
	}
	if payload != nil {
		m["payload"] = payload
	}
	claims := map[string]any{"mercure": m}
	if expires > 0 {
		claims["exp"] = now.Add(expires).Unix()
	}
	return claims
}

// accessClaims are the claims of a Mercure 1.0 access token (RFC 9068, with
// RFC 9396 authorization_details): one detail for publish and one for
// subscribe, the payload going with subscribe.
func accessClaims(issuer, audience, subject, clientID, matchType string, publish, subscribe []string, payload any,
	expires time.Duration, now time.Time) map[string]any {
	jti := make([]byte, 16)
	_, _ = rand.Read(jti)
	details := []any{}
	for _, g := range []struct {
		action string
		topics []string
	}{{"publish", publish}, {"subscribe", subscribe}} {
		if len(g.topics) == 0 {
			continue
		}
		ms := make([]any, 0, len(g.topics))
		for _, m := range g.topics {
			ms = append(ms, map[string]string{"match": m, "match_type": matchType})
		}
		d := map[string]any{"type": detailType, "actions": []string{g.action}, "topics": ms}
		if g.action == "subscribe" && payload != nil {
			d["payload"] = payload
		}
		details = append(details, d)
	}
	if subject == "" {
		subject = clientID
	}
	return map[string]any{
		"iss":                   issuer,
		"aud":                   audience,
		"sub":                   subject,
		"client_id":             clientID,
		"iat":                   now.Unix(),
		"exp":                   now.Add(expires).Unix(),
		"jti":                   base64.RawURLEncoding.EncodeToString(jti),
		"authorization_details": details,
	}
}
