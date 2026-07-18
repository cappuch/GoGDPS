package crypto

import "strconv"

// XORCipher replicates the PHP XORCipher class used by Geometry Dash.
func XORCipher(plaintext string, key int) string {
	keyBytes := textToASCII(strconv.Itoa(key))
	input := textToASCII(plaintext)
	keySize := len(keyBytes)
	out := make([]byte, len(input))
	for i, b := range input {
		out[i] = byte(b ^ keyBytes[i%keySize])
	}
	return string(out)
}

func textToASCII(text string) []int {
	out := make([]int, len(text))
	for i := 0; i < len(text); i++ {
		out[i] = int(text[i])
	}
	return out
}
