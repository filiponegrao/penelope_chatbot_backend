package tools

import (
	"crypto/sha512"
	"encoding/hex"
	"math/rand"
	"time"
)

const numbers = "0123456789"
const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func EncryptTextSHA512(text string) string {
	sum := sha512.Sum512([]byte(text))
	return hex.EncodeToString(sum[:])
}

func RandomNumbers(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = numbers[seededRand.Intn(len(numbers))]
	}
	return string(b)
}

func RandomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
