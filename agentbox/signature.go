package agentbox

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"
)

// Operation represents a file operation
type Operation string

const (
	OperationRead  Operation = "read"
	OperationWrite Operation = "write"
)

// Signature represents a file URL signature
type Signature struct {
	Signature  string
	Expiration *int64 // Unix timestamp or nil
}

// GetSignature generates a v1 signature for sandbox file URLs
// This matches the Python SDK's get_signature() function
func GetSignature(
	path string,
	operation Operation,
	user string,
	envdAccessToken string,
	expirationInSeconds *int,
) (*Signature, error) {
	if envdAccessToken == "" {
		return nil, fmt.Errorf("access token is not set and signature cannot be generated")
	}

	var expiration *int64
	if expirationInSeconds != nil {
		exp := time.Now().Unix() + int64(*expirationInSeconds)
		expiration = &exp
	}

	var raw string
	if expiration == nil {
		raw = fmt.Sprintf("%s:%s:%s:%s", path, operation, user, envdAccessToken)
	} else {
		raw = fmt.Sprintf("%s:%s:%s:%s:%d", path, operation, user, envdAccessToken, *expiration)
	}

	hash := sha256.Sum256([]byte(raw))
	encoded := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])

	return &Signature{
		Signature:  fmt.Sprintf("v1_%s", encoded),
		Expiration: expiration,
	}, nil
}

