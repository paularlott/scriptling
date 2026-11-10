package mercure

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/paularlott/gossiper/client"
	"github.com/paularlott/scriptling/conversion"
	scriptlingerrors "github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// tokenSpec is what a token says: the key that signs it, its claims and how long it lasts.
type tokenSpec struct {
	key       []byte
	algorithm string
	draft07   bool
	issuer    string
	audience  string
	subject   string
	clientID  string
	matchType string
	publish   []string
	subscribe []string
	payload   any
	ttl       time.Duration
}

// mint signs a token issued at now.
func (s *tokenSpec) mint(now time.Time) (string, error) {
	if s.draft07 {
		claims := mercureClaims(s.publish, s.subscribe, s.payload, s.ttl, now)
		if s.subject != "" {
			claims["sub"] = s.subject
		}
		return signToken(s.key, s.algorithm, "JWT", claims)
	}
	return signToken(s.key, s.algorithm, "at+jwt", accessClaims(s.issuer, s.audience, s.subject, s.clientID, s.matchType,
		s.publish, s.subscribe, s.payload, s.ttl, now))
}

// tokenArgs are the keyword arguments of a token, and the names create_token
// and Client give them.
var tokenArgs = []string{"publish", "subscribe", "payload", "issuer", "audience", "match_type", "subject",
	"client_id", "algorithm", "draft07"}

// parseTokenSpec reads a token's arguments. ttlName is the argument that
// holds its lifetime ("expires" for create_token, "token_ttl" for Client);
// grantAll gives a token without publish or subscribe every topic for both
// (a client holding the key could mint that anyway).
func parseTokenSpec(kwargs object.Kwargs, key, audience, ttlName string, grantAll bool) (*tokenSpec, object.Object) {
	s := &tokenSpec{key: []byte(key), audience: audience}
	var errObj object.Object
	if s.publish, errObj = selectors(kwargs, "publish"); errObj != nil {
		return nil, errObj
	}
	if s.subscribe, errObj = selectors(kwargs, "subscribe"); errObj != nil {
		return nil, errObj
	}
	if s.publish == nil && s.subscribe == nil {
		if !grantAll {
			return nil, scriptlingerrors.NewValueError("create_token: give publish or subscribe selectors")
		}
		s.publish, s.subscribe = []string{"*"}, []string{"*"}
	}
	if v := kwargs.Get("payload"); v != nil {
		if _, null := v.(*object.Null); !null {
			g, errObj := conversion.ToGoLimited(v)
			if errObj != nil {
				return nil, errObj
			}
			s.payload = g
		}
	}
	ttl, errObj := kwargs.GetFloat(ttlName, 3600)
	if errObj != nil {
		return nil, errObj
	}
	if ttl < 0 {
		return nil, scriptlingerrors.NewValueError("%s must not be negative", ttlName)
	}
	s.ttl = time.Duration(ttl * float64(time.Second))
	if s.algorithm, errObj = kwargs.GetString("algorithm", "HS256"); errObj != nil {
		return nil, errObj
	}
	if s.subject, errObj = optionalString(kwargs, "subject"); errObj != nil {
		return nil, errObj
	}
	if s.draft07, errObj = kwargs.GetBool("draft07", false); errObj != nil {
		return nil, errObj
	}
	if s.draft07 {
		if s.audience != "" {
			return nil, scriptlingerrors.NewValueError("create_token: a draft-07 token has no audience")
		}
		for _, k := range []string{"issuer", "client_id", "match_type"} {
			if kwargs.Has(k) {
				return nil, scriptlingerrors.NewValueError("create_token: %s is for Mercure 1.0 tokens, not draft-07", k)
			}
		}
	} else {
		if s.audience == "" {
			return nil, scriptlingerrors.NewValueError("create_token: a Mercure 1.0 token needs the hub's URL as its " +
				"audience (create_token(key, hub_url, ...)), or draft07=True for a draft-07 token")
		}
		if s.ttl <= 0 {
			return nil, scriptlingerrors.NewValueError("create_token: a Mercure 1.0 token must expire")
		}
		if s.payload != nil {
			if _, ok := s.payload.(map[string]any); !ok {
				return nil, scriptlingerrors.NewValueError("create_token: a Mercure 1.0 payload must be a dict")
			}
		}
		if s.issuer, errObj = kwargs.GetString("issuer", defaultIssuer); errObj != nil {
			return nil, errObj
		}
		if s.issuer == "" {
			return nil, scriptlingerrors.NewValueError("create_token: issuer must not be empty")
		}
		if s.clientID, errObj = kwargs.GetString("client_id", "scriptling"); errObj != nil {
			return nil, errObj
		}
		if s.matchType, errObj = kwargs.GetString("match_type", "exact"); errObj != nil {
			return nil, errObj
		}
		if s.matchType != "exact" && s.matchType != "urlpattern" {
			return nil, scriptlingerrors.NewValueError(`create_token: match_type must be "exact" or "urlpattern"`)
		}
	}
	if _, err := s.mint(time.Now()); err != nil { // the key and algorithm are checked once, here
		return nil, scriptlingerrors.NewValueError("create_token: %s", err.Error())
	}
	return s, nil
}

// tokenSource is a client's token: one it mints from the hub's key and
// renews, or one the script gives (and replaces with set_token).
type tokenSource struct {
	mu      sync.Mutex
	spec    *tokenSpec // nil: a token the script gives
	token   string
	renewAt time.Time // when a minted token is replaced; zero: never (no expiry)
	now     func() time.Time
}

func newMintingSource(spec *tokenSpec) *tokenSource {
	return &tokenSource{spec: spec, now: time.Now}
}

func newStaticSource(token string) *tokenSource {
	return &tokenSource{token: token, now: time.Now}
}

// get returns the token for a request or a stream. A minted token is renewed
// once half its life is gone, so every request and stream gets one with at
// least half its life ahead; a hub that ends a stream as its token expires
// sees it reopened with a fresh one.
func (t *tokenSource) get(context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.spec == nil {
		return t.token, nil
	}
	now := t.now()
	if t.token == "" || (!t.renewAt.IsZero() && !now.Before(t.renewAt)) {
		tok, err := t.spec.mint(now)
		if err != nil {
			return "", err
		}
		t.token = tok
		if t.spec.ttl > 0 {
			t.renewAt = now.Add(t.spec.ttl / 2)
		}
	}
	return t.token, nil
}

// set replaces a token the script gave.
func (t *tokenSource) set(token string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.spec != nil {
		return errors.New("mercure: this client makes its own tokens from key=; set_token is for a token given with token=")
	}
	t.token = token
	return nil
}

// minting reports whether the source makes its own tokens.
func (t *tokenSource) minting() bool { return t.spec != nil }

// explain adds to a 401 from the hub why it may have come and what to do.
func (t *tokenSource) explain(err error) error {
	var e *client.Error
	if !errors.As(err, &e) || e.Status != http.StatusUnauthorized {
		return err
	}
	if t.minting() {
		return fmt.Errorf("%w (the hub refused the client's token: check that key= is the hub's key, and for "+
			"Mercure 1.0 that the hub trusts the issuer and that its URL is the audience)", err)
	}
	t.mu.Lock()
	given := t.token != ""
	t.mu.Unlock()
	if !given {
		return err
	}
	return fmt.Errorf("%w (the hub refused the token: expired, or not for this hub; with key= the client makes "+
		"and renews its own tokens, or give it a new one with set_token())", err)
}
