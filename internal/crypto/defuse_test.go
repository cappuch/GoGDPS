package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestDecryptProtectedAccountSaveVectors(t *testing.T) {
	pass := "testpass"
	innerKey := make([]byte, 32)
	for i := range innerKey {
		innerKey[i] = byte(i + 1)
	}
	innerKeyASCII := mustSaveChecksummed(keyHeader, innerKey)
	passHash := sha256.Sum256([]byte(pass))
	encInner, err := encryptCiphertextTest([]byte(innerKeyASCII), passHash[:], true)
	if err != nil {
		t.Fatal(err)
	}
	protectedASCII := mustSaveChecksummed(passwordKeyHeader, encInner)
	plaintext := "H4sIAAAAAAAA/encrypted-save-placeholder"
	encSave, err := encryptCiphertextTest([]byte(plaintext), innerKey, false)
	if err != nil {
		t.Fatal(err)
	}
	saveHex := hex.EncodeToString(encSave)
	got, err := DecryptProtectedAccountSave(saveHex, pass, protectedASCII)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != plaintext {
		t.Fatalf("got %q want %q", got, plaintext)
	}
}

func mustSaveChecksummed(header, payload []byte) string {
	checked := append(append([]byte{}, header...), payload...)
	sum := sha256.Sum256(checked)
	return hex.EncodeToString(append(checked, sum[:]...))
}

func encryptCiphertextTest(plaintext, secret []byte, isPassword bool) ([]byte, error) {
	salt := make([]byte, saltByteSize)
	iv := make([]byte, blockByteSize)
	for i := range salt {
		salt[i] = byte(i)
	}
	for i := range iv {
		iv[i] = byte(200 + i)
	}
	var ekey, akey []byte
	if isPassword {
		ekey, akey = deriveKeysFromPassword(secret, salt)
	} else {
		ekey, akey = deriveKeysFromKey(secret, salt)
	}
	block, err := aes.NewCipher(ekey)
	if err != nil {
		return nil, err
	}
	encrypted := make([]byte, len(plaintext))
	cipher.NewCTR(block, iv).XORKeyStream(encrypted, plaintext)
	out := append(append(append([]byte{}, ciphertextHeader...), salt...), iv...)
	out = append(out, encrypted...)
	return append(out, hmacSHA256(out, akey)...), nil
}
