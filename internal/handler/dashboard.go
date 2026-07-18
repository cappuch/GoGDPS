package handler

import (
	"fmt"
	"math"
	"net/http"
	"strings"

	"gogdps/internal/dashboard"
	"gogdps/internal/service"
)

type DashboardHandler struct {
	auth      *service.AuthService
	identity  *service.IdentityService
	dashboard *service.DashboardService
	songs     *service.SongsService
	staticDir string
}

func NewDashboardHandler(auth *service.AuthService, identity *service.IdentityService, dash *service.DashboardService, songs *service.SongsService, staticDir string) *DashboardHandler {
	return &DashboardHandler{auth: auth, identity: identity, dashboard: dash, songs: songs, staticDir: staticDir}
}

func (h *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/dashboard/incl/") {
		http.StripPrefix("/dashboard/", http.FileServer(http.Dir(h.staticDir))).ServeHTTP(w, r)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/dashboard/")
	path = strings.TrimSuffix(path, ".php")

	if path == "incl" || strings.HasPrefix(path, "incl/") {
		http.StripPrefix("/dashboard/", http.FileServer(http.Dir(h.staticDir))).ServeHTTP(w, r)
		return
	}

	accountID := dashboard.AccountID(r)
	userName, _ := h.identity.GetAccountName(r.Context(), accountID)
	hasMod, _ := h.identity.CheckPermission(r.Context(), accountID, "dashboardModTools")
	render := dashboard.NewRenderer(r)
	l := render.Locale

	switch path {
	case "", "index":
		h.index(w, r, render, accountID, userName, hasMod)
	case "login/login":
		h.login(w, r, render, accountID)
	case "login/logout":
		h.logout(w, r, render, accountID)
	case "lang/switchLang":
		h.switchLang(w, r, render, accountID, userName, hasMod)
	case "account/unlisted":
		h.unlisted(w, r, render, accountID, userName, hasMod, l)
	case "stats/dailyTable":
		h.dailyTable(w, r, render, accountID, userName, hasMod, l)
	case "stats/modActions":
		h.modActions(w, r, render, accountID, userName, hasMod, l)
	case "stats/modActionsList":
		h.modActionsList(w, r, render, accountID, userName, hasMod, l)
	case "stats/packTable":
		h.packTable(w, r, render, accountID, userName, hasMod, l)
	case "stats/gauntletTable":
		h.gauntletTable(w, r, render, accountID, userName, hasMod, l)
	case "reupload/songAdd":
		h.songAdd(w, r, render, accountID, userName, hasMod, l)
	case "uploadGJComment21":
		http.Redirect(w, r, "/uploadGJComment21.php", http.StatusFound)
	case "errors/404":
		http.Error(w, "404 Not Found", http.StatusNotFound)
	case "errors/418":
		http.Error(w, "418 I'm a teapot", http.StatusTeapot)
	default:
		http.NotFound(w, r)
	}
}

func (h *DashboardHandler) index(w http.ResponseWriter, r *http.Request, render *dashboard.Renderer, accountID int, userName string, hasMod bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	c1, c2, err := h.dashboard.HomeCharts(r.Context())
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	content := `<p>Welcome to the GDPS dashboard. Please choose a tool above.
<br>DISCLAIMER: THIS AREA IS UNDER HEAVY DEVELOPEMENT, DON'T EXPECT MUCH STUFF TO WORK
<br>Legend: (N) = Not Working, (T) = Links to the legacy tool version
<br><div class="chart-container" style="position: relative; height:30vh; width:80vw"><canvas id="levelsChart"></canvas></div>
<br><div class="chart-container" style="position: relative; height:30vh; width:80vw"><canvas id="levelsChart2"></canvas></div></p>`
	content += render.GenerateLineChart("levelsChart", "Levels Uploaded", c1)
	content += render.GenerateLineChart("levelsChart2", "Levels Uploaded", c2)
	render.PrintPage(w, r, content, false, "home", accountID, userName, hasMod)
}

func (h *DashboardHandler) login(w http.ResponseWriter, r *http.Request, render *dashboard.Renderer, accountID int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if accountID != 0 {
		render.PrintBox(w, r, "<h1>Login</h1><p>You are already logged in. <a href='..'>Click here to continue</a></p>", "", true, accountID, "", false)
		return
	}
	if r.Method == http.MethodPost && r.FormValue("userName") != "" {
		_ = r.ParseForm()
		status, err := h.auth.ValidateUsernamePassword(r.Context(), r.FormValue("userName"), r.FormValue("password"))
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		if status != 1 {
			render.PrintBox(w, r, "<h1>Login</h1><p>Invalid username or password. <a href=''>Click here to try again.</a>", "", true, 0, "", false)
			return
		}
		id, err := h.identity.GetAccountIDFromName(r.Context(), r.FormValue("userName"))
		if err != nil || id == 0 {
			render.PrintBox(w, r, "<h1>Login</h1><p>An error has occured: Invalid accountID. <a href=''>Click here to try again.</a>", "", true, 0, "", false)
			return
		}
		dashboard.SetAccountID(w, id)
		if ref := r.FormValue("ref"); ref != "" {
			http.Redirect(w, r, ref, http.StatusFound)
			return
		}
		if ref := r.Referer(); ref != "" {
			http.Redirect(w, r, ref, http.StatusFound)
			return
		}
		render.PrintBox(w, r, "<h1>Login</h1><p>You are now logged in. <a href='..'>Please click here to continue.</a></p>", "", true, id, "", false)
		return
	}
	form := `<form action="" method="post"><div class="form-group"><label>Username</label>
<input type="text" class="form-control" name="userName" placeholder="Enter username"></div>
<div class="form-group"><label>Password</label>
<input type="password" class="form-control" name="password" placeholder="Password"></div>`
	if ref := r.Referer(); ref != "" {
		form += `<input type="hidden" name="ref" value="` + ref + `">`
	}
	form += `<button type="submit" class="btn btn-primary">Log In</button></form>`
	render.PrintBox(w, r, "<h1>Login</h1>"+form, "", true, 0, "", false)
}

func (h *DashboardHandler) logout(w http.ResponseWriter, r *http.Request, render *dashboard.Renderer, accountID int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	dashboard.ClearAccountID(w)
	if ref := r.Referer(); ref != "" {
		http.Redirect(w, r, ref, http.StatusFound)
		return
	}
	render.PrintBox(w, r, "<h1>Login</h1><p>You are now logged out. <a href='..'>Click here to continue</a></p>", "", true, 0, "", false)
}

func (h *DashboardHandler) switchLang(w http.ResponseWriter, r *http.Request, render *dashboard.Renderer, accountID int, userName string, hasMod bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	lang := r.URL.Query().Get("lang")
	if lang != "" && isAlpha(lang) {
		dashboard.SetLang(w, lang)
		if ref := r.Referer(); ref != "" {
			http.Redirect(w, r, ref, http.StatusFound)
			return
		}
		render.PrintBox(w, r, "<p>Language changed. <a href='index.php'>Click here to continue</a></p>", "", true, accountID, userName, hasMod)
		return
	}
	render.PrintBox(w, r, "Invalid language. <a href='..'>Click here to continue</a>", "", true, accountID, userName, hasMod)
}

func (h *DashboardHandler) unlisted(w http.ResponseWriter, r *http.Request, render *dashboard.Renderer, accountID int, userName string, hasMod bool, l *dashboard.Locale) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if accountID == 0 {
		http.Redirect(w, r, "/dashboard/login/login.php", http.StatusFound)
		return
	}
	offset, page := dashboard.PageNum(r)
	rows, total, err := h.dashboard.UnlistedLevels(r.Context(), accountID, offset)
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	table := fmt.Sprintf(`<table class="table table-inverse"><thead><tr>
<th>%s</th><th>%s</th><th>%s</th><th>%s</th></tr></thead><tbody>%s</tbody></table>`,
		l.T("ID"), l.T("name"), l.T("stars"), l.T("userCoins"), rows)
	pageCount := int(math.Ceil(float64(total) / 10))
	if pageCount < 1 {
		pageCount = 1
	}
	bottom := render.GenerateBottomRow(r.URL.Path, pageCount, page, dashboard.QueryValues(r))
	render.PrintPage(w, r, table+bottom, true, "browse", accountID, userName, hasMod)
}

func (h *DashboardHandler) dailyTable(w http.ResponseWriter, r *http.Request, render *dashboard.Renderer, accountID int, userName string, hasMod bool, l *dashboard.Locale) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	offset, page := dashboard.PageNum(r)
	rows, total, err := h.dashboard.DailyTable(r.Context(), offset)
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	table := fmt.Sprintf(`<table class="table table-inverse"><thead><tr><th>#</th>
<th>%s</th><th>%s</th><th>%s</th><th>%s</th><th>%s</th><th>%s</th></tr></thead><tbody>%s</tbody></table>`,
		l.T("ID"), l.T("name"), l.T("author"), l.T("stars"), l.T("userCoins"), l.T("time"), rows)
	pageCount := int(math.Ceil(float64(total) / 10))
	if pageCount < 1 {
		pageCount = 1
	}
	bottom := render.GenerateBottomRow(r.URL.Path, pageCount, page, dashboard.QueryValues(r))
	render.PrintPage(w, r, table+bottom, true, "stats", accountID, userName, hasMod)
}

func (h *DashboardHandler) modActions(w http.ResponseWriter, r *http.Request, render *dashboard.Renderer, accountID int, userName string, hasMod bool, l *dashboard.Locale) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	rows, err := h.dashboard.ModActionsSummary(r.Context())
	if err != nil {
		msg := fmt.Sprintf(l.T("errorNoAccWithPerm"), "toolsModactions")
		render.PrintBox(w, r, msg, "stats", true, accountID, userName, hasMod)
		return
	}
	table := fmt.Sprintf(`<table class="table table-inverse"><thead><tr><th>#</th>
<th>%s</th><th>%s</th><th>%s</th><th>%s</th></tr></thead><tbody>%s</tbody></table>`,
		l.T("mod"), l.T("count"), l.T("ratedLevels"), l.T("lastSeen"), rows)
	render.PrintPage(w, r, table, true, "stats", accountID, userName, hasMod)
}

func (h *DashboardHandler) modActionsList(w http.ResponseWriter, r *http.Request, render *dashboard.Renderer, accountID int, userName string, hasMod bool, l *dashboard.Locale) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	offset, page := dashboard.PageNum(r)
	rows, total, err := h.dashboard.ModActionsList(r.Context(), l, offset)
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	table := fmt.Sprintf(`<table class="table table-inverse"><tr><th>#</th><th>%s</th><th>%s</th><th>%s</th><th>%s</th><th>%s</th><th>%s</th></tr>%s</table>`,
		l.T("mod"), l.T("action"), l.T("value"), l.T("value2"), l.T("level"), l.T("time"), rows)
	pageCount := int(math.Ceil(float64(total) / 10))
	if pageCount < 1 {
		pageCount = 1
	}
	bottom := render.GenerateBottomRow(r.URL.Path, pageCount, page, dashboard.QueryValues(r))
	render.PrintPage(w, r, table+bottom, true, "browse", accountID, userName, hasMod)
}

func (h *DashboardHandler) packTable(w http.ResponseWriter, r *http.Request, render *dashboard.Renderer, accountID int, userName string, hasMod bool, l *dashboard.Locale) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	offset, page := dashboard.PageNum(r)
	rows, total, err := h.dashboard.PackTable(r.Context(), l, offset)
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	table := fmt.Sprintf(`<table class="table table-inverse"><thead><tr><th>#</th><th>%s</th><th>%s</th><th>%s</th><th>%s</th></tr></thead><tbody>%s</tbody></table>`,
		l.T("name"), l.T("stars"), l.T("coins"), l.T("levels"), rows)
	pageCount := int(math.Ceil(float64(total) / 10))
	if pageCount < 1 {
		pageCount = 1
	}
	bottom := render.GenerateBottomRow(r.URL.Path, pageCount, page, dashboard.QueryValues(r))
	render.PrintPage(w, r, table+bottom, true, "stats", accountID, userName, hasMod)
}

func (h *DashboardHandler) gauntletTable(w http.ResponseWriter, r *http.Request, render *dashboard.Renderer, accountID int, userName string, hasMod bool, l *dashboard.Locale) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	offset, page := dashboard.PageNum(r)
	rows, total, err := h.dashboard.GauntletTable(r.Context(), l, offset)
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	table := fmt.Sprintf(`<table class="table table-inverse"><thead><tr><th>#</th><th>%s</th><th>%s</th></tr></thead><tbody>%s</tbody></table>`,
		l.T("name"), l.T("levels"), rows)
	pageCount := int(math.Ceil(float64(total) / 10))
	if pageCount < 1 {
		pageCount = 1
	}
	bottom := render.GenerateBottomRow(r.URL.Path, pageCount, page, dashboard.QueryValues(r))
	render.PrintPage(w, r, table+bottom, true, "stats", accountID, userName, hasMod)
}

func (h *DashboardHandler) songAdd(w http.ResponseWriter, r *http.Request, render *dashboard.Renderer, accountID int, userName string, hasMod bool, l *dashboard.Locale) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodPost && r.FormValue("url") != "" {
		_ = r.ParseForm()
		result, err := h.songs.ReuploadURL(r.Context(), r.FormValue("url"))
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		if strings.HasPrefix(result, "-") {
			errDesc := l.T("songAddError" + result)
			content := fmt.Sprintf("<h1>%s</h1><p>%s %s (%s)</p><a class='btn btn-primary btn-block' href='%s'>%s</a>",
				l.T("songAdd"), l.T("errorGeneric"), result, errDesc, r.URL.Path, l.T("tryAgainBTN"))
			render.PrintBox(w, r, content, "reupload", true, accountID, userName, hasMod)
			return
		}
		content := fmt.Sprintf("<h1>%s</h1><p>Song Reuploaded: %s</p><a class='btn btn-primary btn-block' href='%s'>%s</a>",
			l.T("songAdd"), result, r.URL.Path, l.T("songAddAnotherBTN"))
		render.PrintBox(w, r, content, "reupload", true, accountID, userName, hasMod)
		return
	}
	form := fmt.Sprintf(`<h1>%s</h1><form action="" method="post"><div class="form-group">
<label for="urlField">%s</label>
<input type="text" class="form-control" id="urlField" name="url" placeholder="%s"></div>
<button type="submit" class="btn btn-primary btn-block">%s</button></form>`,
		l.T("songAdd"), l.T("songAddUrlFieldLabel"), l.T("songAddUrlFieldPlaceholder"), l.T("reuploadBTN"))
	render.PrintBox(w, r, form, "reupload", true, accountID, userName, hasMod)
}

func isAlpha(s string) bool {
	for _, c := range s {
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
			return false
		}
	}
	return len(s) > 0
}
