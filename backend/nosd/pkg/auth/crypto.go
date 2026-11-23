package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"

	"golang.org/x/crypto/argon2"
)

type ArgonParams struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLen     uint32
	KeyLen      uint32
}

var DefaultParams = ArgonParams{Memory: 64 * 1024, Iterations: 1, Parallelism: 4, SaltLen: 16, KeyLen: 32}

func HashPassword(p ArgonParams, password string) (string, error) {
	salt := make([]byte, p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLen)
	// simple encoding: base64(salt)$base64(hash)
	return base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(hash), nil
}

func VerifyPassword(p ArgonParams, encoded, password string) bool {
	parts := []byte(encoded)
	sep := -1
	for i, b := range parts {
		if b == '$' {
			sep = i
			break
		}
	}
	if sep <= 0 {
		return false
	}
	saltB, err1 := base64.RawStdEncoding.DecodeString(string(parts[:sep]))
	hashB, err2 := base64.RawStdEncoding.DecodeString(string(parts[sep+1:]))
	if err1 != nil || err2 != nil {
		return false
	}
	calc := argon2.IDKey([]byte(password), saltB, p.Iterations, p.Memory, p.Parallelism, uint32(len(hashB)))
	return subtle.ConstantTimeCompare(calc, hashB) == 1
}
