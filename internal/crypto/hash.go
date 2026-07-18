package crypto

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strconv"
)

const (
	hashSalt1 = "xI25fpAapCQg"
	hashSalt2 = "oC36fpYaPtdg"
	hashSalt3 = "pC26fpYaQCtg"
)

// GenMulti hashes a list of levels for getGJLevels responses.
func GenMulti(levels []LevelHashInput) string {
	hash := ""
	for _, lvl := range levels {
		id := strconv.Itoa(lvl.LevelID)
		hash += string(id[0]) + string(id[len(id)-1]) + strconv.Itoa(lvl.Stars) + strconv.Itoa(lvl.Coins)
	}
	return sha1Hex(hash + hashSalt1)
}

type LevelHashInput struct {
	LevelID int
	Stars   int
	Coins   int
}

// GenSolo hashes a single level string for download responses.
func GenSolo(levelString string) string {
	length := len(levelString)
	if length < 41 {
		return sha1Hex(levelString + hashSalt1)
	}
	hash := make([]byte, 40)
	for i := range hash {
		hash[i] = '?'
	}
	m := length / 40
	for i := 40; i > 0; i-- {
		hash[i-1] = levelString[i*m]
	}
	return sha1Hex(string(hash) + hashSalt1)
}

// GenSolo2 hashes auxiliary download metadata.
func GenSolo2(s string) string {
	return sha1Hex(s + hashSalt1)
}

// GenSolo3 hashes map pack data.
func GenSolo3(s string) string {
	return sha1Hex(s + hashSalt2)
}

// GenSolo4 hashes gauntlet/chest data.
func GenSolo4(s string) string {
	return sha1Hex(s + hashSalt3)
}

// GenPack hashes map pack list IDs (stars/coins from each pack).
func GenPack(packs []LevelHashInput) string {
	hash := ""
	for _, p := range packs {
		id := strconv.Itoa(p.LevelID)
		hash += string(id[0]) + string(id[len(id)-1]) + strconv.Itoa(p.Stars) + strconv.Itoa(p.Coins)
	}
	return sha1Hex(hash + hashSalt1)
}

func sha1Hex(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func GJP2FromPassword(password string) string {
	sum := sha1.Sum([]byte(password + "mI29fmAnxgTs"))
	return fmt.Sprintf("%x", sum)
}

// GenSeed2NoXor mirrors GenerateHash::genSeed2noXor for level uploads.
func GenSeed2NoXor(levelString string) string {
	hash := []byte("aaaaa")
	length := len(levelString)
	if length == 0 {
		return sha1Hex(string(hash) + hashSalt1)
	}
	divided := length / 50
	if divided < 1 {
		divided = 1
	}
	p := 0
	for k := 0; k < length; k += divided {
		if p > 49 {
			break
		}
		hash[p] = levelString[k]
		p++
	}
	return sha1Hex(string(hash) + hashSalt1)
}
