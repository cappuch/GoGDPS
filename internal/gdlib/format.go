package gdlib

import (
	"fmt"
	"math"
	"strconv"
)

// UserProfile holds fields used in GD colon-delimited user responses.
type UserProfile struct {
	UserName      string
	UserID        int
	Coins         int
	UserCoins     int
	Icon          int
	Color1        int
	Color2        int
	Color3        int
	IconType      int
	Special       int
	ExtID         string
	Stars         int
	CreatorPoints float64
	Demons        int
	Diamonds      int
	Moons         int
}

func NumericExtID(extID string) string {
	if isNumeric(extID) {
		return extID
	}
	return "0"
}

// FormatLeaderboardEntry formats a user for getGJScores / getGJCreators responses.
func FormatLeaderboardEntry(u UserProfile, rank int) string {
	ext := NumericExtID(u.ExtID)
	cp := int(math.Floor(u.CreatorPoints))
	return fmt.Sprintf(
		"1:%s:2:%d:13:%d:17:%d:6:%d:9:%d:10:%d:11:%d:51:%d:14:%d:15:%d:16:%s:3:%d:8:%d:4:%d:7:%s:46:%d:52:%d",
		u.UserName, u.UserID, u.Coins, u.UserCoins, rank,
		u.Icon, u.Color1, u.Color2, u.Color3, u.IconType, u.Special, ext,
		u.Stars, cp, u.Demons, ext, u.Diamonds, u.Moons,
	)
}

// FormatLeaderboardEntryLegacy omits color3/moons for older friend leaderboard responses.
func FormatLeaderboardEntryLegacy(u UserProfile, rank int) string {
	ext := NumericExtID(u.ExtID)
	cp := int(math.Floor(u.CreatorPoints))
	return fmt.Sprintf(
		"1:%s:2:%d:13:%d:17:%d:6:%d:9:%d:10:%d:11:%d:14:%d:15:%d:16:%s:3:%d:8:%d:4:%d:7:%s:46:%d",
		u.UserName, u.UserID, u.Coins, u.UserCoins, rank,
		u.Icon, u.Color1, u.Color2, u.IconType, u.Special, ext,
		u.Stars, cp, u.Demons, ext, u.Diamonds,
	)
}

// FormatLevelScoreEntry formats a level score row for getGJLevelScores.
func FormatLevelScoreEntry(u UserProfile, percent, place, coins int, uploadDate string) string {
	ext := NumericExtID(u.ExtID)
	return fmt.Sprintf(
		"1:%s:2:%d:9:%d:10:%d:11:%d:51:%d:14:%d:15:%d:16:%s:3:%d:6:%d:13:%d:42:%s",
		u.UserName, u.UserID, u.Icon, u.Color1, u.Color2, u.Color3,
		u.IconType, u.Special, ext, percent, place, coins, uploadDate,
	)
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	_, err := strconv.Atoi(s)
	return err == nil
}
