package crypto

import (
	"crypto/rand"
	"encoding/hex"
	mathrand "math/rand"
)

// RandomString mirrors mainLib::randomString (hex-encoded random bytes).
func RandomString(length int) string {
	if length <= 0 {
		length = 6
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		const chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		out := make([]byte, length*2)
		for i := range out {
			out[i] = chars[mathrand.Intn(len(chars))]
		}
		return string(out)
	}
	return hex.EncodeToString(buf)
}
