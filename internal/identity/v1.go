package identity

import (
	"crypto/ed25519"
	"fmt"
	"strings"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/mr-tron/base58"
)

const (
	JWT_HEADER                 = "X-Restate-Jwt-V1"
	SchemeV1   SignatureScheme = "v1"
)

type KeySetV1 = map[string]ed25519.PublicKey

func validateV1(keySet KeySetV1, path string, headers map[string][]string) error {
	switch len(headers[JWT_HEADER]) {
	case 0:
		return fmt.Errorf("v1 signature scheme expects the following headers: [%s]", JWT_HEADER)
	case 1:
	default:
		return fmt.Errorf("unexpected multi-value JWT header: %v", headers[JWT_HEADER])
	}

	token, err := jwt.Parse(headers[JWT_HEADER][0], func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"]
		if !ok {
			return nil, fmt.Errorf("Token missing 'kid' header field")
		}

		kidS, ok := kid.(string)
		if !ok {
			return nil, fmt.Errorf("Token 'kid' header field was not a string: %v", kid)
		}

		key, ok := keySet[kidS]
		if !ok {
			return nil, fmt.Errorf("Key ID %s is not present in key set", kid)
		}

		return key, nil
	}, jwt.WithValidMethods([]string{"EdDSA"}), jwt.WithAudience(path), jwt.WithExpirationRequired())
	if err != nil {
		return fmt.Errorf("failed to validate v1 request identity jwt: %w", err)
	}

	nbf, _ := token.Claims.GetNotBefore()
	if nbf == nil {
		// jwt library only validates nbf if its present, so we should check it was present
		return fmt.Errorf("'nbf' claim is missing in v1 request identity jwt")
	}

	return nil
}

func ParseKeySetV1(keys []string) (KeySetV1, error) {
	out := make(KeySetV1, len(keys))
	for _, key := range keys {
		if !strings.HasPrefix(key, "publickeyv1_") {
			return nil, fmt.Errorf("v1 public key must start with 'publickeyv1_'")
		}

		pubBytes, err := base58.Decode(key[len("publickeyv1_"):])
		if err != nil {
			return nil, fmt.Errorf("v1 public key must be valid base58: %w", err)
		}

		if len(pubBytes) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("v1 public key must have exactly %d bytes, found %d", ed25519.PublicKeySize, len(pubBytes))
		}

		out[key] = ed25519.PublicKey(pubBytes)
	}

	return out, nil
}
