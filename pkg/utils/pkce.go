package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// GenerateRandomString 生成指定长度的随机字符串 (用于 verifier 和 state)
func GenerateRandomString(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	var result strings.Builder
	for _, bVal := range b {
		result.WriteByte(charset[int(bVal)%len(charset)])
	}
	return result.String(), nil
}

// GenerateCodeChallenge 基于 verifier 生成 S256 Challenge 字符串
// 算法：Base64UrlEncode(SHA256(ASCII(verifier)))
func GenerateCodeChallenge(verifier string) string {
	h := sha256.New()
	h.Write([]byte(verifier))
	// Etsy 要求使用 RawURLEncoding (不带填充符=)
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
