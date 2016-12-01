package randString

import (
	"crypto/rand"
	"encoding/base64"
)

// Taken from @elithrar at elithrar.github.io

// Get cryptosecure bytes
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n) // array to hold the random bytes
	_, err := rand.Read(b)
	// Fail if we don't get back the correct length
	if err != nil {
		return nil, err
	}

	return b, nil
}

// Get a url-safe cryptosecure string
func GenerateRandomString(n int) (string, error) {
	b, err := GenerateRandomBytes(n)
	return base64.URLEncoding.EncodeToString(b), err
}
