// Package auth verifies Clerk browser sessions for the speech administration API.
package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	// ErrInvalidToken means the supplied Clerk session cannot be trusted.
	ErrInvalidToken = errors.New("auth: invalid Clerk session token")
	// ErrNotOwner means Clerk confirmed the session but it is not the configured owner.
	ErrNotOwner = errors.New("auth: Clerk user is not the configured Google owner")
	// ErrUnavailable means Clerk could not be safely consulted. Callers must fail closed.
	ErrUnavailable = errors.New("auth: Clerk service unavailable")
)

const (
	defaultAPIBase        = "https://api.clerk.com/v1"
	maxResponseBody       = 8 << 20
	maxTokenBytes         = 16 << 10
	keyRefreshMinInterval = time.Minute
	keyFailureBackoff     = 5 * time.Second
	ownerCacheTTL         = time.Minute
	maxOwnerCacheEntries  = 128
	tokenLeeway           = 10 * time.Second
)

var hostnameRE = regexp.MustCompile(`^(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)(?:\.(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?))+$`)

// Config contains the Clerk instance settings. The optional HTTP fields are only
// transport endpoints for tests; production defaults remain Clerk's HTTPS APIs.
type Config struct {
	SecretKey          string
	PublishableKey     string
	AllowedGoogleEmail string
	AuthorizedParties  []string

	HTTPClient *http.Client
	APIBase    string
	JWKSURL    string
	Issuer     string
}

// Owner is the Clerk identity permitted to administer Speech.
type Owner struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type cacheEntry struct {
	owner     Owner
	expiresAt time.Time
}

// Client validates an origin-bound Clerk JWT, then verifies the owner through
// Clerk's Backend API. It never accepts browser-provided profile data.
type Client struct {
	secretKey      string
	publishableKey string
	frontendAPI    string
	allowedEmail   string
	authorized     map[string]bool
	http           *http.Client
	apiBase        string
	jwksURL        string
	issuer         string

	now func() time.Time

	keysMu      sync.Mutex
	keys        map[string]*rsa.PublicKey
	lastSuccess time.Time
	lastFailure time.Time

	ownersMu sync.Mutex
	owners   map[string]cacheEntry
}

// New creates a verifier. It deliberately makes no network request so a
// deployment can start before the first browser sign-in.
func New(cfg Config) (*Client, error) {
	secret := strings.TrimSpace(cfg.SecretKey)
	if secret == "" {
		return nil, errors.New("auth: Clerk secret key is required")
	}
	email := strings.TrimSpace(cfg.AllowedGoogleEmail)
	if email == "" {
		return nil, errors.New("auth: allowed Google email is required")
	}
	fapi, err := frontendAPI(cfg.PublishableKey)
	if err != nil {
		return nil, err
	}
	parties := make(map[string]bool, len(cfg.AuthorizedParties))
	for _, party := range cfg.AuthorizedParties {
		party = normalizeOrigin(party)
		if party != "" {
			parties[party] = true
		}
	}
	if len(parties) == 0 {
		return nil, errors.New("auth: at least one authorized party is required")
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	apiBase := strings.TrimRight(strings.TrimSpace(cfg.APIBase), "/")
	if apiBase == "" {
		apiBase = defaultAPIBase
	}
	jwksURL := strings.TrimSpace(cfg.JWKSURL)
	if jwksURL == "" {
		jwksURL = "https://" + fapi + "/.well-known/jwks.json"
	}
	issuer := strings.TrimSpace(cfg.Issuer)
	if issuer == "" {
		issuer = "https://" + fapi
	}
	return &Client{
		secretKey: secret, publishableKey: strings.TrimSpace(cfg.PublishableKey),
		frontendAPI: fapi, allowedEmail: email, authorized: parties, http: hc,
		apiBase: apiBase, jwksURL: jwksURL, issuer: issuer, now: time.Now,
		keys: map[string]*rsa.PublicKey{}, owners: map[string]cacheEntry{},
	}, nil
}

// PublishableKey returns Clerk's intentionally public browser key.
func (c *Client) PublishableKey() string { return c.publishableKey }

// FrontendAPI returns the Clerk Frontend API hostname encoded in the public key.
func (c *Client) FrontendAPI() string { return c.frontendAPI }

// VerifyOwner accepts only a Clerk RS256 session token minted for an explicitly
// authorized browser origin and bound to the verified Google owner.
func (c *Client) VerifyOwner(ctx context.Context, token string) (Owner, error) {
	claims, err := c.verifyToken(ctx, token)
	if err != nil {
		return Owner{}, err
	}
	if owner, ok := c.cachedOwner(claims.Subject); ok {
		return owner, nil
	}
	owner, err := c.readOwner(ctx, claims.Subject)
	if err != nil {
		return Owner{}, err
	}
	c.cacheOwner(owner)
	return owner, nil
}

type sessionClaims struct {
	AuthorizedParty string `json:"azp"`
	jwt.RegisteredClaims
}

func (c *Client) verifyToken(ctx context.Context, raw string) (sessionClaims, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxTokenBytes {
		return sessionClaims{}, ErrInvalidToken
	}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithIssuer(c.issuer), jwt.WithExpirationRequired(),
		jwt.WithLeeway(tokenLeeway), jwt.WithTimeFunc(c.now),
	)
	var unavailable error
	parsed, err := parser.ParseWithClaims(raw, &sessionClaims{}, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("missing key id")
		}
		key, keyErr := c.keyFor(ctx, kid)
		if errors.Is(keyErr, ErrUnavailable) {
			unavailable = keyErr
		}
		return key, keyErr
	})
	if unavailable != nil {
		return sessionClaims{}, unavailable
	}
	if err != nil || !parsed.Valid {
		return sessionClaims{}, ErrInvalidToken
	}
	claims, ok := parsed.Claims.(*sessionClaims)
	if !ok || strings.TrimSpace(claims.Subject) == "" || !c.authorized[normalizeOrigin(claims.AuthorizedParty)] {
		return sessionClaims{}, ErrInvalidToken
	}
	return *claims, nil
}

func (c *Client) keyFor(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	c.keysMu.Lock()
	defer c.keysMu.Unlock()
	if key := c.keys[kid]; key != nil {
		return key, nil
	}
	now := c.now()
	if !c.lastFailure.IsZero() && now.Sub(c.lastFailure) < keyFailureBackoff {
		return nil, ErrUnavailable
	}
	if c.lastFailure.IsZero() && !c.lastSuccess.IsZero() && now.Sub(c.lastSuccess) < keyRefreshMinInterval {
		return nil, ErrInvalidToken
	}
	keys, err := c.fetchJWKS(ctx)
	if err != nil {
		c.lastFailure = now
		return nil, err
	}
	c.keys, c.lastFailure, c.lastSuccess = keys, time.Time{}, now
	if key := c.keys[kid]; key != nil {
		return key, nil
	}
	return nil, ErrInvalidToken
}

type jwks struct {
	Keys []struct {
		Kty string `json:"kty"`
		Kid string `json:"kid"`
		Use string `json:"use"`
		Alg string `json:"alg"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

func (c *Client) fetchJWKS(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.jwksURL, nil)
	if err != nil {
		return nil, ErrUnavailable
	}
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return nil, ErrUnavailable
	}
	var set jwks
	if err := json.NewDecoder(io.LimitReader(res.Body, maxResponseBody)).Decode(&set); err != nil {
		return nil, ErrUnavailable
	}
	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, key := range set.Keys {
		if key.Kty != "RSA" || key.Kid == "" || (key.Use != "" && key.Use != "sig") || (key.Alg != "" && key.Alg != "RS256") {
			continue
		}
		pub, err := rsaFromJWK(key.N, key.E)
		if err == nil {
			keys[key.Kid] = pub
		}
	}
	if len(keys) == 0 {
		return nil, ErrUnavailable
	}
	return keys, nil
}

func rsaFromJWK(n, e string) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(n, "="))
	if err != nil {
		return nil, err
	}
	eb, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(e, "="))
	if err != nil || len(nb) == 0 || len(eb) == 0 || len(eb) > 8 {
		return nil, errors.New("invalid RSA JWK")
	}
	exponent := new(big.Int).SetBytes(eb)
	if !exponent.IsInt64() || exponent.Int64() < 3 || exponent.Int64() > int64(^uint32(0)) {
		return nil, errors.New("invalid RSA exponent")
	}
	modulus := new(big.Int).SetBytes(nb)
	if modulus.BitLen() < 2048 {
		return nil, errors.New("RSA modulus too small")
	}
	return &rsa.PublicKey{N: modulus, E: int(exponent.Int64())}, nil
}

type clerkUser struct {
	ID             string `json:"id"`
	Banned         bool   `json:"banned"`
	Locked         bool   `json:"locked"`
	PrimaryEmailID string `json:"primary_email_address_id"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	Emails         []struct {
		ID           string `json:"id"`
		Email        string `json:"email_address"`
		Verification struct {
			Status string `json:"status"`
		} `json:"verification"`
	} `json:"email_addresses"`
	ExternalAccounts []struct {
		Provider     string `json:"provider"`
		Email        string `json:"email_address"`
		Verification struct {
			Status string `json:"status"`
		} `json:"verification"`
	} `json:"external_accounts"`
}

func (c *Client) readOwner(ctx context.Context, id string) (Owner, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Owner{}, ErrNotOwner
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBase+"/users/"+url.PathEscape(id), nil)
	if err != nil {
		return Owner{}, ErrUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+c.secretKey)
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return Owner{}, ErrUnavailable
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return Owner{}, ErrNotOwner
	}
	if res.StatusCode/100 != 2 {
		return Owner{}, ErrUnavailable
	}
	var user clerkUser
	if err := json.NewDecoder(io.LimitReader(res.Body, maxResponseBody)).Decode(&user); err != nil {
		return Owner{}, ErrUnavailable
	}
	if user.ID != id || user.Banned || user.Locked {
		return Owner{}, ErrNotOwner
	}
	var email string
	for _, candidate := range user.Emails {
		if candidate.ID == user.PrimaryEmailID && candidate.Verification.Status == "verified" && strings.EqualFold(candidate.Email, c.allowedEmail) {
			email = candidate.Email
			break
		}
	}
	if email == "" {
		return Owner{}, ErrNotOwner
	}
	for _, account := range user.ExternalAccounts {
		if (account.Provider == "oauth_google" || account.Provider == "google") && account.Verification.Status == "verified" && strings.EqualFold(account.Email, email) {
			return Owner{ID: id, Email: email, Name: strings.TrimSpace(user.FirstName + " " + user.LastName)}, nil
		}
	}
	return Owner{}, ErrNotOwner
}

func (c *Client) cachedOwner(id string) (Owner, bool) {
	c.ownersMu.Lock()
	defer c.ownersMu.Unlock()
	entry, ok := c.owners[id]
	if !ok || !c.now().Before(entry.expiresAt) {
		delete(c.owners, id)
		return Owner{}, false
	}
	return entry.owner, true
}

func (c *Client) cacheOwner(owner Owner) {
	c.ownersMu.Lock()
	defer c.ownersMu.Unlock()
	if len(c.owners) >= maxOwnerCacheEntries {
		for id, entry := range c.owners {
			if !c.now().Before(entry.expiresAt) {
				delete(c.owners, id)
			}
		}
		if len(c.owners) >= maxOwnerCacheEntries {
			for id := range c.owners {
				delete(c.owners, id)
				break
			}
		}
	}
	c.owners[owner.ID] = cacheEntry{owner: owner, expiresAt: c.now().Add(ownerCacheTTL)}
}

func frontendAPI(key string) (string, error) {
	key = strings.TrimSpace(key)
	var encoded string
	switch {
	case strings.HasPrefix(key, "pk_live_"):
		encoded = strings.TrimPrefix(key, "pk_live_")
	case strings.HasPrefix(key, "pk_test_"):
		encoded = strings.TrimPrefix(key, "pk_test_")
	default:
		return "", errors.New("auth: Clerk publishable key is invalid")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(encoded, "="))
	}
	if err != nil {
		return "", errors.New("auth: Clerk publishable key is invalid")
	}
	host := strings.ToLower(strings.TrimSuffix(string(raw), "$"))
	if !hostnameRE.MatchString(host) {
		return "", errors.New("auth: Clerk publishable key does not encode a hostname")
	}
	return host, nil
}

func normalizeOrigin(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return ""
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return ""
	}
	return strings.ToLower(u.Scheme + "://" + u.Host)
}
