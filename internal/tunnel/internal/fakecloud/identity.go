package fakecloud

import (
	"crypto/ed25519"
	"crypto/rand"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/mr-tron/base58"
)

// IdentityKey is a test-side request-identity signer. It mints an Ed25519
// keypair in the publickeyv1_<base58> format and signs v1 identity JWTs the way
// the Restate runtime does (EdDSA; aud = the service-relative request path;
// exp/iat/nbf with 60s leeway), so tests can prove the SDK-delegated
// verification end to end.
type IdentityKey struct {
	// PublicKey is the publickeyv1_... string to configure as SigningPublicKey.
	PublicKey string

	priv ed25519.PrivateKey
}

// GenerateIdentityKey creates a fresh signing key.
func GenerateIdentityKey() (*IdentityKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &IdentityKey{
		PublicKey: "publickeyv1_" + base58.Encode(pub),
		priv:      priv,
	}, nil
}

// Sign returns a v1 identity JWT whose audience is aud (the request path).
func (k *IdentityKey) Sign(aud string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"aud": aud,
		"nbf": now.Add(-60 * time.Second).Unix(),
		"iat": now.Unix(),
		"exp": now.Add(60 * time.Second).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = k.PublicKey
	return token.SignedString(k.priv)
}
