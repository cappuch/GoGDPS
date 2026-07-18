package dashboard

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

const sessionCookie = "gdps_dashboard_account"

type Locale struct {
	lang string
}

func NewLocale(r *http.Request) *Locale {
	lang := "EN"
	if c, err := r.Cookie("lang"); err == nil && isAlpha(c.Value) {
		lang = strings.ToUpper(c.Value)
	}
	return &Locale{lang: lang}
}

func (l *Locale) T(key string) string {
	if l.lang == "TEST" {
		return "lnf:" + key
	}
	if v, ok := enStrings[key]; ok {
		return v
	}
	return "lnf:" + key
}

func (l *Locale) Tf(key string, args ...any) string {
	return fmt.Sprintf(l.T(key), args...)
}

func isAlpha(s string) bool {
	for _, c := range s {
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
			return false
		}
	}
	return len(s) > 0
}

var enStrings = map[string]string{
	"homeNavbar": "Home", "accountManagement": "Account Management", "changePassword": "Change Password",
	"changeUsername": "Change Username", "unlistedLevels": "Unlisted Levels", "modTools": "Mod Tools",
	"leaderboardBan": "Ban A User", "packManage": "Create a Map Pack", "reuploadSection": "Reupload",
	"levelReupload": "Level Reupload", "songAdd": "Add a Song", "browse": "Browse", "statsSection": "Stats",
	"dailyTable": "Daily Levels", "modActions": "Mod Actions", "gauntletTable": "List of Gauntlets",
	"packTable": "List of Map Packs", "leaderboardTime": "Leaderboards Progress", "language": "Language",
	"loginHeader": "Welcome, %s", "logout": "Log Out", "login": "Log In", "reuploadBTN": "Reupload",
	"errorGeneric": "An error has occured:", "tryAgainBTN": "Try Again",
	"songAddUrlFieldLabel": "Song URL: (Direct or Dropbox Links Only)",
	"songAddUrlFieldPlaceholder": "Song URL", "songAddAnotherBTN": "Another Song",
	"songAddError-2": "Invalid URL", "songAddError-3": "This song has been reuploaded already",
	"songAddError-4": "This song isn't reuploadable",
	"ID": "ID", "stars": "Stars", "coins": "Coins", "accounts": "Accounts", "levels": "Levels",
	"songs": "Songs", "author": "Creator", "name": "Name", "userCoins": "User Coins", "time": "Time",
	"deletedLevel": "Deleted Level", "mod": "Moderator", "count": "Amount", "ratedLevels": "Rated Levels",
	"lastSeen": "Last Time Online", "level": "Level", "pageInfo": "Showing page %d of %d",
	"first": "First", "previous": "Previous", "next": "Next", "last": "Last", "go": "Go",
	"action": "Action", "value": "1st Value", "value2": "2nd Value",
	"modAction1": "Rated a level", "modAction2": "Un/featured a level", "modAction3": "Un/verified coins",
	"modAction4": "Un/epiced a level", "modAction5": "Set as daily feature", "modAction6": "Deleted a level",
	"modAction7": "Creator change", "modAction8": "Renamed a level", "modAction9": "Changed level password",
	"modAction10": "Changed demon difficulty", "modAction11": "Shared CP", "modAction12": "Un/published",
	"modAction13": "Changed level description", "modAction14": "Enabled/disabled LDM",
	"modAction15": "Leaderboard un/banned", "modAction16": "Song ID change",
	"errorNoAccWithPerm": "Error: No accounts with the '%s' permission have been found",
}

func AccountID(r *http.Request) int {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return 0
	}
	var id int
	fmt.Sscanf(c.Value, "%d", &id)
	return id
}

func SetAccountID(w http.ResponseWriter, accountID int) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: fmt.Sprintf("%d", accountID),
		Path: "/", MaxAge: int((365 * 24 * time.Hour).Seconds()),
	})
}

func ClearAccountID(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
}

func SetLang(w http.ResponseWriter, lang string) {
	http.SetCookie(w, &http.Cookie{
		Name: "lang", Value: strings.ToUpper(lang), Path: "/",
		MaxAge: int((365 * 24 * time.Hour).Seconds()),
	})
}

func ConvertToDate(ts int64) string {
	return time.Unix(ts, 0).Format("02/01/2006 15:04:05")
}

func ModActionName(l *Locale, t string) string {
	return l.T("modAction" + t)
}
