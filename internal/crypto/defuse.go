package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"

	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/pbkdf2"
)

var (
	ErrDefuseDecrypt = errors.New("defuse: decryption failed")

	keyHeader            = []byte{0xDE, 0xF0, 0x00, 0x00}
	passwordKeyHeader    = []byte{0xDE, 0xF1, 0x00, 0x00}
	ciphertextHeader     = []byte{0xDE, 0xF5, 0x02, 0x00}
	headerVersionSize    = 4
	saltByteSize         = 32
	blockByteSize        = 16
	macByteSize          = 32
	checksumByteSize     = 32
	minimumCiphertextLen = 84
	pbkdf2Iterations     = 100_000
	encInfo              = "DefusePHP|V2|KeyForEncryption"
	authInfo             = "DefusePHP|V2|KeyForAuthentication"
)

// DecryptProtectedAccountSave mirrors syncGJAccount20 defuse-crypto handling.
func DecryptProtectedAccountSave(saveData, password, protectedKeyEncoded string) (string, error) {
	protectedKeyEncoded = strings.TrimSpace(protectedKeyEncoded)
	if protectedKeyEncoded == "" {
		return "", ErrDefuseDecrypt
	}
	encKey, err := loadChecksummedASCII(passwordKeyHeader, protectedKeyEncoded)
	if err != nil {
		return "", err
	}
	passHash := sha256.Sum256([]byte(password))
	innerKeyASCII, err := decryptCiphertext(encKey, passHash[:], true)
	if err != nil {
		return "", err
	}
	keyBytes, err := loadChecksummedASCII(keyHeader, string(innerKeyASCII))
	if err != nil {
		return "", err
	}
	plain, err := decryptCiphertext([]byte(strings.TrimSpace(saveData)), keyBytes, false)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func loadChecksummedASCII(expectedHeader []byte, s string) ([]byte, error) {
	s = trimTrailingWhitespace(s)
	raw, err := hex.DecodeString(s)
	if err != nil {
		return nil, ErrDefuseDecrypt
	}
	if len(raw) < headerVersionSize+checksumByteSize {
		return nil, ErrDefuseDecrypt
	}
	if !bytesEqual(raw[:headerVersionSize], expectedHeader) {
		return nil, ErrDefuseDecrypt
	}
	checked := raw[:len(raw)-checksumByteSize]
	sum := raw[len(raw)-checksumByteSize:]
	if !bytesEqual(sha256Sum(checked), sum) {
		return nil, ErrDefuseDecrypt
	}
	return raw[headerVersionSize : len(raw)-checksumByteSize], nil
}

func decryptCiphertext(ciphertext, secret []byte, isPassword bool) ([]byte, error) {
	if len(ciphertext) >= minimumCiphertextLen && bytesEqual(ciphertext[:headerVersionSize], ciphertextHeader) {
		return decryptRawCiphertext(ciphertext, secret, isPassword)
	}
	decoded, err := hex.DecodeString(string(ciphertext))
	if err != nil {
		return nil, ErrDefuseDecrypt
	}
	return decryptRawCiphertext(decoded, secret, isPassword)
}

func decryptRawCiphertext(ciphertext, secret []byte, isPassword bool) ([]byte, error) {
	if len(ciphertext) < minimumCiphertextLen {
		return nil, ErrDefuseDecrypt
	}
	if !bytesEqual(ciphertext[:headerVersionSize], ciphertextHeader) {
		return nil, ErrDefuseDecrypt
	}
	salt := ciphertext[headerVersionSize : headerVersionSize+saltByteSize]
	iv := ciphertext[headerVersionSize+saltByteSize : headerVersionSize+saltByteSize+blockByteSize]
	macStart := len(ciphertext) - macByteSize
	encrypted := ciphertext[headerVersionSize+saltByteSize+blockByteSize : macStart]
	mac := ciphertext[macStart:]

	var ekey, akey []byte
	if isPassword {
		ekey, akey = deriveKeysFromPassword(secret, salt)
	} else {
		ekey, akey = deriveKeysFromKey(secret, salt)
	}
	message := ciphertext[:macStart]
	if !hmac.Equal(mac, hmacSHA256(message, akey)) {
		return nil, ErrDefuseDecrypt
	}
	block, err := aes.NewCipher(ekey)
	if err != nil {
		return nil, err
	}
	ctr := cipher.NewCTR(block, iv)
	plain := make([]byte, len(encrypted))
	ctr.XORKeyStream(plain, encrypted)
	return plain, nil
}

func deriveKeysFromPassword(password, salt []byte) (ekey, akey []byte) {
	prehash := sha256.Sum256(password)
	prekey := pbkdf2.Key(prehash[:], salt, pbkdf2Iterations, 32, sha256.New)
	return deriveHKDFPair(prekey, salt)
}

func deriveKeysFromKey(key, salt []byte) (ekey, akey []byte) {
	return deriveHKDFPair(key, salt)
}

func deriveHKDFPair(secret, salt []byte) (ekey, akey []byte) {
	ekey = make([]byte, 32)
	akey = make([]byte, 32)
	encHKDF := hkdf.New(sha256.New, secret, salt, []byte(encInfo))
	_, _ = io.ReadFull(encHKDF, ekey)
	authHKDF := hkdf.New(sha256.New, secret, salt, []byte(authInfo))
	_, _ = io.ReadFull(authHKDF, akey)
	return ekey, akey
}

func hmacSHA256(message, key []byte) []byte {
	m := hmac.New(sha256.New, key)
	_, _ = m.Write(message)
	return m.Sum(nil)
}

func sha256Sum(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	out := 0
	for i := range a {
		out |= int(a[i] ^ b[i])
	}
	return out == 0
}

func trimTrailingWhitespace(s string) string {
	for len(s) > 0 {
		switch s[len(s)-1] {
		case 0, '\t', '\n', '\r', ' ':
			s = s[:len(s)-1]
		default:
			return s
		}
	}
	return s
}
