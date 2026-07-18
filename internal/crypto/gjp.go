package crypto

import (
	"encoding/base64"
	"strings"
)

const GJPKey = 37526

// DecodeGJP decodes a client GJP token into the plaintext password.
func DecodeGJP(gjp string) (string, error) {
	decoded := strings.ReplaceAll(gjp, "_", "/")
	decoded = strings.ReplaceAll(decoded, "-", "+")
	raw, err := base64.StdEncoding.DecodeString(decoded)
	if err != nil {
		return "", err
	}
	return XORCipher(string(raw), GJPKey), nil
}
