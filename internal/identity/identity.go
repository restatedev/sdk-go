package identity

import "fmt"

const SIGNATURE_SCHEME_HEADER = "X-Restate-Signature-Scheme"

type SignatureScheme string

var (
	SchemeUnsigned     SignatureScheme = "unsigned"
	errMissingIdentity                 = fmt.Errorf("request has no identity")
)

func ValidateRequestIdentity(keySet KeySetV1, path string, headers map[string][]string) error {
	switch len(headers[SIGNATURE_SCHEME_HEADER]) {
	case 0:
		return errMissingIdentity
	case 1:
		switch SignatureScheme(headers[SIGNATURE_SCHEME_HEADER][0]) {
		case SchemeV1:
			return validateV1(keySet, path, headers)
		case SchemeUnsigned:
			return errMissingIdentity
		default:
			return fmt.Errorf("unexpected signature scheme %v, allowed values are [%s %s]", headers[SIGNATURE_SCHEME_HEADER][0], SchemeUnsigned, SchemeV1)
		}
	default:
		return fmt.Errorf("unexpected multi-value signature scheme header: %v", headers[SIGNATURE_SCHEME_HEADER])
	}
}
