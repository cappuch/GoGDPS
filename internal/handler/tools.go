package handler

import (
	"net/http"
	"strings"

	"gogdps/internal/captcha"
	"gogdps/internal/sanitize"
	"gogdps/internal/service"
)

type ToolsHandler struct {
	auth      *service.AuthService
	captcha   *captcha.Validator
	songs     *service.SongsService
	tools     *service.ToolsService
	cron      *service.CronService
	localHost string
}

func NewToolsHandler(auth *service.AuthService, captcha *captcha.Validator, songs *service.SongsService, tools *service.ToolsService, cron *service.CronService, localHost string) *ToolsHandler {
	return &ToolsHandler{auth: auth, captcha: captcha, songs: songs, tools: tools, cron: cron, localHost: localHost}
}

func (h *ToolsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/tools/")
	path = strings.TrimSuffix(path, ".php")

	switch path {
	case "", "index":
		h.index(w)
	case "account/activateAccount":
		h.activateAccount(w, r)
	case "account/registerAccount":
		h.registerAccount(w, r)
	case "account/changePassword":
		h.changePassword(w, r)
	case "account/changeUsername":
		h.changeUsername(w, r)
	case "songAdd":
		h.songAdd(w, r)
	case "leaderboardsBan":
		h.leaderboardsBan(w, r, true)
	case "leaderboardsUnban":
		h.leaderboardsBan(w, r, false)
	case "linkAcc":
		h.linkAcc(w, r)
	case "cron/cron":
		h.cronJob(w, r, "")
	case "cron/fixcps", "cron/autoban", "cron/friendsLeaderboard", "cron/removeBlankLevels", "cron/songsCount", "cron/fixnames":
		h.cronJob(w, r, strings.TrimPrefix(path, "cron/"))
	case "cleanup/deleteUnused":
		h.deleteUnused(w, r)
	case "packCreate":
		h.packCreate(w, r)
	case "levelReupload":
		h.levelReupload(w, r)
	case "levelToGD":
		h.levelToGD(w, r)
	case "addQuests":
		h.addQuests(w, r)
	case "revertLikes":
		h.revertLikes(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *ToolsHandler) index(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<a href="/dashboard/">Check out the dashboard here</a>
<h1>Account management tools:</h1><ul>
<li><a href="account/activateAccount">activateAccount</a></li>
<li><a href="account/registerAccount">registerAccount</a></li>
<li><a href="account/changePassword">changePassword</a></li>
<li><a href="account/changeUsername">changeUsername</a></li>
</ul>
<h1>Upload related tools:</h1><ul>
<li><a href="songAdd">songAdd</a></li>
<li><a href="linkAcc">linkAcc</a></li>
<li><a href="levelReupload">levelReupload</a></li>
<li><a href="levelToGD">levelToGD</a></li>
<li><a href="packCreate">packCreate</a></li>
<li><a href="addQuests">addQuests</a></li>
<li><a href="leaderboardsBan">leaderboardsBan</a></li>
<li><a href="leaderboardsUnban">leaderboardsUnban</a></li>
</ul>
<h1>Stats tools:</h1><ul>
<li><a href="stats/stats">stats</a></li>
<li><a href="stats/vipList">vipList</a></li>
<li><a href="stats/top24h">top24h</a></li>
<li><a href="stats/songList">songList</a></li>
<li><a href="stats/reportList">reportList</a></li>
<li><a href="stats/packTable">packTable</a></li>
<li><a href="stats/noLogIn">noLogIn</a></li>
<li><a href="stats/modActions">modActions</a></li>
<li><a href="stats/dailyTable">dailyTable</a></li>
<li><a href="stats/unlisted">unlisted</a></li>
<li><a href="stats/suggestList">suggestList</a></li>
</ul>
<h1>Bot tools:</h1><ul>
<li><a href="bot/levelSearchBot">levelSearchBot</a></li>
<li><a href="bot/playerStatsBot">playerStatsBot</a></li>
<li><a href="bot/leaderboardsBot">leaderboardsBot</a></li>
</ul>
<h1>The cron job</h1><ul>
<li><a href="cron/cron">cron.php</a></li>
</ul>`))
}

func (h *ToolsHandler) songAdd(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodPost && r.FormValue("songlink") != "" {
		_ = r.ParseForm()
		ok, err := h.captcha.Validate(r.FormValue("h-captcha-response"))
		if err != nil {
			http.Error(w, "captcha error", http.StatusInternalServerError)
			return
		}
		if !ok {
			_, _ = w.Write([]byte("Invalid captcha response"))
			return
		}
		result, err := h.songs.ReuploadURL(r.Context(), r.FormValue("songlink"))
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		switch result {
		case "-4":
			_, _ = w.Write([]byte("This URL doesn't point to a valid audio file."))
		case "-3":
			_, _ = w.Write([]byte("This song already exists in our database."))
		case "-2":
			_, _ = w.Write([]byte("The download link isn't a valid URL"))
		default:
			_, _ = w.Write([]byte("Song reuploaded: <b>" + result + "</b><hr>"))
		}
		return
	}
	_, _ = w.Write([]byte(`<b>Direct links</b> or <b>Dropbox links</b> only accepted, <b><font size="5">NO YOUTUBE LINKS</font></b><br>
<form action="songAdd.php" method="post">
Link: <input type="text" name="songlink"><br>` + h.captcha.DisplayHTML() + `
<input type="submit" value="Add Song"></form>`))
}

func (h *ToolsHandler) leaderboardsBan(w http.ResponseWriter, r *http.Request, ban bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodPost && r.FormValue("userName") != "" && r.FormValue("password") != "" && r.FormValue("userID") != "" {
		_ = r.ParseForm()
		msg, err := h.tools.LeaderboardsBan(r.Context(),
			sanitize.Remove(r.FormValue("userName")),
			sanitize.Remove(r.FormValue("password")),
			sanitize.Remove(r.FormValue("userID")), ban)
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(msg))
		return
	}
	action := "leaderboardsBan.php"
	submit := "Ban"
	if !ban {
		action = "leaderboardsUnban.php"
		submit = "unBan"
	}
	_, _ = w.Write([]byte(`<form action="` + action + `" method="post">Your Username: <input type="text" name="userName">
<br>Your Password: <input type="password" name="password">
<br>Target UserID: <input type="text" name="userID">
<br><input type="submit" value="` + submit + `"></form>`))
}

func (h *ToolsHandler) linkAcc(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodPost &&
		r.FormValue("userhere") != "" && r.FormValue("passhere") != "" &&
		r.FormValue("usertarg") != "" && r.FormValue("passtarg") != "" {
		_ = r.ParseForm()
		server := r.FormValue("server")
		if server == "" {
			server = "http://www.boomlings.com/database/accounts/loginGJAccount.php"
		}
		msg, err := h.tools.LinkAccount(r.Context(), service.LinkAccountInput{
			LocalUser:  sanitize.Remove(r.FormValue("userhere")),
			LocalPass:  sanitize.Remove(r.FormValue("passhere")),
			TargetUser: sanitize.Remove(r.FormValue("usertarg")),
			TargetPass: sanitize.Remove(r.FormValue("passtarg")),
			ServerURL:  server,
			Debug:      r.FormValue("debug") == "1",
			LocalHost:  h.localHost,
		})
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(msg))
		return
	}
	_, _ = w.Write([]byte(`<html><head><title>ACCOUNT LINKING</title></head><body>
<form action="linkAcc.php" method="post">Your password for the target server is NOT saved, it's used for one-time verification purposes only.
<h3>This server</h3>
Username: <input type="text" name="userhere"><br>
Password: <input type="password" name="passhere"><br>
<h3>Target server</h3>
Username: <input type="text" name="usertarg"><br>
Password: <input type="password" name="passtarg"><br>
<details>
<summary>Advanced options</summary>
URL: <input type="text" name="server" value="http://www.boomlings.com/database/accounts/loginGJAccount.php"><br>
Debug Mode (0=off, 1=on): <input type="text" name="debug" value="0"><br>
</details>
<input type="submit" value="Link Accounts"></form></body></html>`))
}

func (h *ToolsHandler) cronJob(w http.ResponseWriter, r *http.Request, job string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var out string
	var err error
	if job == "" {
		out, err = h.cron.Run(r.Context())
	} else {
		out, err = h.cron.RunJob(r.Context(), job)
	}
	if err != nil {
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	_, _ = w.Write([]byte(out))
}

func (h *ToolsHandler) deleteUnused(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	out, err := h.tools.DeleteUnused(r.Context())
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write([]byte("<hr>" + out))
}

func (h *ToolsHandler) activateAccount(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodPost && r.FormValue("userName") != "" && r.FormValue("password") != "" {
		_ = r.ParseForm()
		ok, err := h.captcha.Validate(r.FormValue("h-captcha-response"))
		if err != nil {
			http.Error(w, "captcha error", http.StatusInternalServerError)
			return
		}
		if !ok {
			_, _ = w.Write([]byte("Invalid captcha response"))
			return
		}
		msg, err := h.auth.ActivateAccount(r.Context(),
			sanitize.Remove(r.FormValue("userName")),
			sanitize.Remove(r.FormValue("password")))
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(msg))
		return
	}
	_, _ = w.Write([]byte(`<form method="post">
Username: <input type="text" name="userName"><br>
Password: <input type="password" name="password"><br>` + h.captcha.DisplayHTML() + `
<input type="submit" value="Activate"></form>`))
}

func (h *ToolsHandler) registerAccount(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodPost &&
		r.FormValue("username") != "" && r.FormValue("email") != "" &&
		r.FormValue("repeatemail") != "" && r.FormValue("password") != "" &&
		r.FormValue("repeatpassword") != "" {
		_ = r.ParseForm()
		result, err := h.auth.WebRegister(r.Context(),
			sanitize.Remove(r.FormValue("username")),
			sanitize.Remove(r.FormValue("password")),
			sanitize.Remove(r.FormValue("repeatpassword")),
			sanitize.Remove(r.FormValue("email")),
			sanitize.Remove(r.FormValue("repeatemail")))
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(result.HTML))
		return
	}
	_, _ = w.Write([]byte(webRegisterPage("")))
}

func (h *ToolsHandler) changePassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodPost &&
		r.FormValue("userName") != "" && r.FormValue("newpassword") != "" &&
		r.FormValue("oldpassword") != "" {
		_ = r.ParseForm()
		msg, err := h.auth.ChangePassword(r.Context(),
			sanitize.Remove(r.FormValue("userName")),
			r.FormValue("oldpassword"),
			sanitize.Remove(r.FormValue("newpassword")))
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(msg))
		return
	}
	_, _ = w.Write([]byte(`<form action="changePassword.php" method="post">Username: <input type="text" name="userName"><br>Old password: <input type="password" name="oldpassword"><br>New password: <input type="password" name="newpassword"><br><input type="submit" value="Change"></form>`))
}

func (h *ToolsHandler) changeUsername(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodPost &&
		r.FormValue("userName") != "" && r.FormValue("newusr") != "" &&
		r.FormValue("password") != "" {
		_ = r.ParseForm()
		msg, err := h.auth.ChangeUsername(r.Context(),
			sanitize.Remove(r.FormValue("userName")),
			sanitize.Remove(r.FormValue("newusr")),
			sanitize.Remove(r.FormValue("password")))
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(msg))
		return
	}
	_, _ = w.Write([]byte(`<form action="changeUsername.php" method="post">Old username: <input type="text" name="userName"><br>New username: <input type="text" name="newusr"><br>Password: <input type="password" name="password"><br><input type="submit" value="Change"></form>`))
}

func webRegisterPage(body string) string {
	if body != "" {
		body += "<br><br>"
	}
	return `<body style="background-color:grey;">` + body + `<form action="registerAccount.php" method="post">Username: <input type="text" name="username" maxlength=15><br>Password: <input type="password" name="password" maxlength=20><br>Repeat Password: <input type="password" name="repeatpassword" maxlength=20><br>Email: <input type="email" name="email" maxlength=50><br>Repeat Email: <input type="email" name="repeatemail" maxlength=50><br><input type="submit" value="Register"></form></body>`
}

func (h *ToolsHandler) packCreate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodPost && r.FormValue("userName") != "" {
		_ = r.ParseForm()
		msg, err := h.tools.PackCreate(r.Context(), service.PackCreateInput{
			UserName: sanitize.Remove(r.FormValue("userName")),
			Password: sanitize.Remove(r.FormValue("password")),
			PackName: sanitize.Remove(r.FormValue("packName")),
			Levels:   sanitize.Remove(r.FormValue("levels")),
			Stars:    sanitize.Remove(r.FormValue("stars")),
			Coins:    sanitize.Remove(r.FormValue("coins")),
			Color:    r.FormValue("color"),
		})
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(msg))
		return
	}
	_, _ = w.Write([]byte(`<form action="packCreate.php" method="post">Username: <input type="text" name="userName">
<br>Password: <input type="password" name="password">
<br>Pack Name: <input type="text" name="packName">
<br>Level IDs: <input type="text" name="levels"> (separate by commas)
<br>Stars: <input type="text" name="stars"> (max 10)
<br>Coins: <input type="text" name="coins"> (max 2)
<br>Color: <input type="color" name="color" value="#ffffff">
<input type="submit" value="Create"></form>`))
}

func (h *ToolsHandler) levelReupload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodPost && r.FormValue("levelid") != "" {
		_ = r.ParseForm()
		msg, err := h.tools.LevelReupload(r.Context(), service.LevelReuploadInput{
			LevelID:   r.FormValue("levelid"),
			ServerURL: r.FormValue("server"),
			Debug:     r.FormValue("debug") == "1",
			LocalHost: h.localHost,
		})
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("<html><head><title>LEVEL REUPLOAD</title></head><body>" + msg + "</body></html>"))
		return
	}
	_, _ = w.Write([]byte(`<html><head><title>LEVEL REUPLOAD</title></head><body>` + levelReuploadFormHTML() + `</body></html>`))
}

func levelReuploadFormHTML() string {
	return `<h4><a href="linkAcc.php">LINKING YOUR ACCOUNT USING linkAcc.php RECOMMENDED</a></h4>
<form action="levelReupload.php" method="post">ID: <input type="text" name="levelid"><br>
<details><summary>Advanced options</summary>
URL: <input type="text" name="server" value="http://www.boomlings.com/database/downloadGJLevel22.php"><br>
Debug Mode (0=off, 1=on): <input type="text" name="debug" value="0"><br>
</details><input type="submit" value="Reupload"></form>`
}

func (h *ToolsHandler) levelToGD(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodPost && r.FormValue("userhere") != "" {
		_ = r.ParseForm()
		msg, err := h.tools.LevelToGD(r.Context(), service.LevelToGDInput{
			LocalUser:  sanitize.Remove(r.FormValue("userhere")),
			LocalPass:  sanitize.Remove(r.FormValue("passhere")),
			TargetUser: sanitize.Remove(r.FormValue("usertarg")),
			TargetPass: sanitize.Remove(r.FormValue("passtarg")),
			LevelID:    sanitize.Remove(r.FormValue("levelID")),
			Server:     r.FormValue("server"),
			Debug:      r.FormValue("debug") == "1",
		})
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("<html><head><title>LEVEL REUPLOAD TO NORMAL GD</title></head><body>" + msg + "</body></html>"))
		return
	}
	_, _ = w.Write([]byte(`<html><head><title>LEVEL REUPLOAD TO NORMAL GD</title></head><body>
<form action="levelToGD.php" method="post">Your password for the target server is NOT saved, it's used for one-time verification purposes only.
<h3>This server</h3>Username: <input type="text" name="userhere"><br>
Password: <input type="password" name="passhere"><br>
Level ID: <input type="text" name="levelID"><br>
<h3>Target server</h3>Username: <input type="text" name="usertarg"><br>
Password: <input type="password" name="passtarg"><br>
<details><summary>Advanced options</summary>
URL: <input type="text" name="server" value="http://www.boomlings.com/database/"><br>
Debug Mode (0=off, 1=on): <input type="text" name="debug" value="0"><br>
</details><input type="submit" value="Reupload"></form></body></html>`))
}

func (h *ToolsHandler) addQuests(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodPost && r.FormValue("userName") != "" {
		_ = r.ParseForm()
		msg, err := h.tools.AddQuest(r.Context(),
			sanitize.Remove(r.FormValue("userName")),
			sanitize.Remove(r.FormValue("password")),
			r.FormValue("type"),
			r.FormValue("amount"),
			r.FormValue("reward"),
			r.FormValue("names"))
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(msg))
		return
	}
	_, _ = w.Write([]byte(`<form action="addQuests.php" method="post">Username: <input type="text" name="userName">
<br>Password: <input type="password" name="password">
<br>Quest Type: <select name="type">
<option value="1">Orbs</option><option value="2">Coins</option><option value="3">Star</option></select>
<br>Amount: <input type="number" name="amount">
<br>Reward: <input type="number" name="reward">
<br>Quest Name: <input type="text" name="names">
<input type="submit" value="Create"></form>`))
}

func (h *ToolsHandler) revertLikes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodPost && r.FormValue("userName") != "" && r.FormValue("password") != "" &&
		r.FormValue("levelID") != "" && r.FormValue("timestamp") != "" {
		_ = r.ParseForm()
		msg, err := h.tools.RevertLikes(r.Context(),
			sanitize.Remove(r.FormValue("userName")),
			sanitize.Remove(r.FormValue("password")),
			sanitize.Remove(r.FormValue("levelID")),
			sanitize.Remove(r.FormValue("timestamp")))
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(msg))
		return
	}
	msg, _ := h.tools.RevertLikes(r.Context(), "", "", "", "")
	_, _ = w.Write([]byte(msg))
}
