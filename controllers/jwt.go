package controllers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"strconv"
)

func getJWTSecret() string {
	secret := getenv("JWT_SECRET", "")
	if secret == "" {
		secret = getenv("PENELOPE_JWT_SECRET", "")
	}
	if secret == "" {
		secret = "CHANGE_ME"
	}
	return secret
}

func signHS256JWT(secret string, claims map[string]any) (string, error) {
	// Header
	header := map[string]any{"alg": "HS256", "typ": "JWT"}
	headB, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	// Payload
	payloadB, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	enc := base64.RawURLEncoding
	unsigned := enc.EncodeToString(headB) + "." + enc.EncodeToString(payloadB)

	h := hmac.New(sha256.New, []byte(secret))
	_, _ = h.Write([]byte(unsigned))
	sig := enc.EncodeToString(h.Sum(nil))
	return unsigned + "." + sig, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getenvInt(k string, def int) int {
	s := getenv(k, "")
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
