// Package auth hashes passwords with argon2id and issues opaque session
// tokens. Only token hashes are ever stored.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// OWASP minimum argon2id profile: 19 MiB, 2 passes, 1 lane.
const (
	memoryKiB = 19 * 1024
	passes    = 2
	lanes     = 1
	keyLen    = 32
	saltLen   = 16
)

var (
	ErrPasswordLength = errors.New("auth: password must be 8 to 128 bytes")
	ErrNameInvalid    = errors.New("auth: name must be 2 to 32 letters, digits, _ or -")
	ErrHashFormat     = errors.New("auth: malformed password hash")
)

var b64 = base64.RawStdEncoding

// HashPassword returns a PHC-style argon2id string.
func HashPassword(password string) (string, error) {
	if len(password) < 8 || len(password) > 128 {
		return "", ErrPasswordLength
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, passes, memoryKiB, lanes, keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memoryKiB, passes, lanes, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// VerifyPassword checks password against a HashPassword string in constant
// time, using the parameters recorded in the hash.
func VerifyPassword(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrHashFormat
	}
	var version int
	var m uint32
	var t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, ErrHashFormat
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil || m == 0 || t == 0 || p == 0 {
		return false, ErrHashFormat
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, ErrHashFormat
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, ErrHashFormat
	}
	got := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// dummyHash is verified against when an account does not exist, so unknown
// names cost the same time as wrong passwords.
var dummyHash, _ = HashPassword("timing-equalizer-not-a-real-password")

// VerifyMissing burns the same work as a real verification.
func VerifyMissing(password string) { _, _ = VerifyPassword(dummyHash, password) }

// ValidName accepts 2–32 runes of letters (including CJK), digits, _ and -.
func ValidName(name string) error {
	n := utf8.RuneCountInString(name)
	if n < 2 || n > 32 || !utf8.ValidString(name) {
		return ErrNameInvalid
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
			return ErrNameInvalid
		}
	}
	return nil
}

// NewToken returns a random session token and the hash to store.
func NewToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, TokenHash(token), nil
}

func TokenHash(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}
