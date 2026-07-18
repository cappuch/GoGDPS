package dashboard

import (
	"fmt"
	"html"
	"net/http"
	"strings"
)

type Renderer struct {
	Locale *Locale
}

func NewRenderer(r *http.Request) *Renderer {
	return &Renderer{Locale: NewLocale(r)}
}

func (d *Renderer) PrintHeader(w http.ResponseWriter, isSubdirectory bool) {
	base := ""
	if isSubdirectory {
		base = `<base href="../">`
	}
	fmt.Fprintf(w, `<!DOCTYPE html><html lang="en"><head><meta charset="utf-8">%s
<script src="https://code.jquery.com/jquery-3.2.1.slim.min.js" crossorigin="anonymous"></script>
<script async src="https://cdnjs.cloudflare.com/ajax/libs/popper.js/1.11.0/umd/popper.min.js" crossorigin="anonymous"></script>
<script async src="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0-beta/js/bootstrap.min.js" crossorigin="anonymous"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/Chart.js/2.7.0/Chart.min.js"></script>
<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0-beta/css/bootstrap.min.css" crossorigin="anonymous">
<link rel="stylesheet" href="incl/cvolton.css">
<link rel="stylesheet" href="incl/font-awesome-4.7.0/css/font-awesome.min.css">
<title>[Beta] GDPS Dashboard</title>
<meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no"></head><body>`, base)
}

func (d *Renderer) PrintFooter(w http.ResponseWriter) {
	_, _ = w.Write([]byte("</body></html>"))
}

func (d *Renderer) PrintNavbar(w http.ResponseWriter, r *http.Request, active string, accountID int, userName string, hasModTools bool) {
	home, account, mod, reupload, stats := "", "", "", "", ""
	switch active {
	case "home":
		home = "active"
	case "account", "browse":
		account = "active"
	case "mod":
		mod = "active"
	case "reupload":
		reupload = "active"
	case "stats":
		stats = "active"
	}
	l := d.Locale
	fmt.Fprintf(w, `<nav class="navbar navbar-expand-lg navbar-dark menubar">
<a class="navbar-brand" href="index.php">GDPS</a>
<button class="navbar-toggler" type="button" data-toggle="collapse" data-target="#navbarNavDropdown">
<span class="navbar-toggler-icon"></span></button>
<div class="collapse navbar-collapse" id="navbarNavDropdown"><ul class="navbar-nav">
<li class="nav-item %s"><a class="nav-link" href="index.php"><i class="fa fa-home"></i> %s</a></li>`,
		home, l.T("homeNavbar"))

	browse := fmt.Sprintf(`<li class="nav-item dropdown %s">
<a class="nav-link dropdown-toggle" href="#" data-toggle="dropdown"><i class="fa fa-folder-open"></i> %s</a>
<div class="dropdown-menu">
<a class="dropdown-item" href="#">%s (N)</a>
<a class="dropdown-item" href="#">%s (N)</a>
<a class="dropdown-item" href="stats/modActionsList.php">%s</a>
<a class="dropdown-item" href="stats/packTable.php">%s</a>
<a class="dropdown-item" href="stats/gauntletTable.php">%s</a>`,
		account, l.T("browse"), l.T("accounts"), l.T("levels"), l.T("modActions"), l.T("packTable"), l.T("gauntletTable"))

	if accountID != 0 {
		fmt.Fprintf(w, `<li class="nav-item dropdown %s">
<a class="nav-link dropdown-toggle" href="#" data-toggle="dropdown"><i class="fa fa-user"></i> %s</a>
<div class="dropdown-menu">
<a class="dropdown-item" href="../tools/account/changePassword.php">%s (T)</a>
<a class="dropdown-item" href="../tools/account/changeUsername.php">%s (T)</a>
<a class="dropdown-item" href="account/unlisted.php">%s</a>
</div></li>%s
<a class="dropdown-item" href="../tools/stats/songList.php">%s (T)</a></div></li>`,
			account, l.T("accountManagement"), l.T("changePassword"), l.T("changeUsername"),
			l.T("unlistedLevels"), browse, l.T("songs"))
		if hasModTools {
			fmt.Fprintf(w, `<li class="nav-item dropdown %s">
<a class="nav-link dropdown-toggle" href="#" data-toggle="dropdown"><i class="fa fa-wrench"></i> %s</a>
<div class="dropdown-menu">
<a class="dropdown-item" href="../tools/leaderboardsBan.php">%s (T)</a>
<a class="dropdown-item" href="../tools/packCreate.php">%s (T)</a>
</div></li>`, mod, l.T("modTools"), l.T("leaderboardBan"), l.T("packManage"))
		}
	} else {
		fmt.Fprintf(w, "%s</div></li>", browse)
	}

	fmt.Fprintf(w, `<li class="nav-item dropdown %s">
<a class="nav-link dropdown-toggle" href="#" data-toggle="dropdown"><i class="fa fa-upload"></i> %s</a>
<div class="dropdown-menu">
<a class="dropdown-item" href="../tools/levelReupload.php">%s (T)</a>
<a class="dropdown-item" href="reupload/songAdd.php">%s</a>
</div></li>
<li class="nav-item dropdown %s">
<a class="nav-link dropdown-toggle" href="#" data-toggle="dropdown"><i class="fa fa-bar-chart"></i> %s</a>
<div class="dropdown-menu">
<a class="dropdown-item" href="stats/dailyTable.php">%s</a>
<a class="dropdown-item" href="stats/modActions.php">%s</a>
<a class="dropdown-item" href="../tools/stats/top24h.php">%s (T)</a>
</div></li></ul><ul class="nav navbar-nav ml-auto">`, reupload, l.T("reuploadSection"), l.T("levelReupload"), l.T("songAdd"),
		stats, l.T("statsSection"), l.T("dailyTable"), l.T("modActions"), l.T("leaderboardTime"))

	fmt.Fprintf(w, `<li class="nav-item dropdown"><a class="nav-link dropdown-toggle" href="#" data-toggle="dropdown">
<i class="fa fa-language"></i> %s</a><div class="dropdown-menu">`, l.T("language"))
	for _, lang := range []string{"CS", "DE", "EE", "EN", "EO", "ES", "GR", "HR", "IT", "PT", "RU", "TH", "TR", "test"} {
		fmt.Fprintf(w, `<a class="dropdown-item" href="lang/switchLang.php?lang=%s">%s</a>`, lang, lang)
	}
	if accountID != 0 {
		fmt.Fprintf(w, `</div></li><li class="nav-item dropdown">
<a class="nav-link dropdown-toggle" href="#" data-toggle="dropdown"><i class="fa fa-user-circle"></i> %s</a>
<div class="dropdown-menu"><a class="dropdown-item" href="login/logout.php"><i class="fa fa-sign-out"></i> %s</a></div></li>`,
			l.Tf("loginHeader", html.EscapeString(userName)), l.T("logout"))
	} else {
		fmt.Fprintf(w, `</div></li><li class="nav-item dropdown">
<a class="nav-link dropdown-toggle" href="#" data-toggle="dropdown"><i class="fa fa-sign-in"></i> %s</a>
<div class="dropdown-menu dropdown-menu-right" style="padding:17px;">
<form action="login/login.php" method="post">
<div class="form-group"><input type="text" class="form-control login-input" name="userName" placeholder="Username"></div>
<div class="form-group"><input type="password" class="form-control login-input" name="password" placeholder="Password"></div>
<button type="submit" class="btn btn-primary btn-block">%s</button></form></div></li>`, l.T("login"), l.T("login"))
	}
	_, _ = w.Write([]byte("</ul></div></nav>"))
}

func (d *Renderer) PrintPage(w http.ResponseWriter, r *http.Request, content string, isSubdirectory bool, navbar string, accountID int, userName string, hasModTools bool) {
	d.PrintHeader(w, isSubdirectory)
	d.PrintNavbar(w, r, navbar, accountID, userName, hasModTools)
	fmt.Fprintf(w, `<div class="container d-flex flex-column"><div class="row fill d-flex justify-content-start content buffer">%s</div></div>`, content)
	d.PrintFooter(w)
}

func (d *Renderer) PrintBox(w http.ResponseWriter, r *http.Request, content, active string, isSubdirectory bool, accountID int, userName string, hasModTools bool) {
	d.PrintHeader(w, isSubdirectory)
	d.PrintNavbar(w, r, active, accountID, userName, hasModTools)
	_, _ = w.Write([]byte(`<div class="container container-box"><div class="card"><div class="card-block buffer">`))
	_, _ = w.Write([]byte(content))
	_, _ = w.Write([]byte(`</div></div></div>`))
	d.PrintFooter(w)
}

func (d *Renderer) GenerateBottomRow(path string, pageCount, actualPage int, query urlValues) string {
	if pageCount < 1 {
		pageCount = 1
	}
	pageMinus := actualPage - 1
	pagePlus := actualPage + 1
	if pageMinus < 1 {
		pageMinus = 1
	}
	base := strings.Split(path, "?")[0]
	out := fmt.Sprintf(`<div>%s</div><div class="btn-group" style="margin-left:auto; margin-right:0;">`,
		d.Locale.Tf("pageInfo", actualPage, pageCount))
	out += fmt.Sprintf(`<a id="first" href="%s?page=1" class="btn btn-outline-secondary"><i class="fa fa-backward"></i> %s</a>`, base, d.Locale.T("first"))
	out += fmt.Sprintf(`<a id="prev" href="%s?page=%d" class="btn btn-outline-secondary"><i class="fa fa-chevron-left"></i> %s</a>`, base, pageMinus, d.Locale.T("previous"))
	out += `<a class="btn btn-outline-secondary" href="#" data-toggle="dropdown">..</a>
<div class="dropdown-menu" style="padding:17px;"><form action="" method="get">
<div class="form-group"><input type="text" class="form-control" name="page" placeholder="#">`
	for k, v := range query {
		if k != "page" {
			out += fmt.Sprintf(`<input type="hidden" name="%s" value="%s">`, html.EscapeString(k), html.EscapeString(v))
		}
	}
	out += fmt.Sprintf(`</div><button type="submit" class="btn btn-primary btn-block">%s</button></form></div>`, d.Locale.T("go"))
	out += fmt.Sprintf(`<a href="%s?page=%d" id="next" class="btn btn-outline-secondary">%s <i class="fa fa-chevron-right"></i></a>`, base, pagePlus, d.Locale.T("next"))
	out += fmt.Sprintf(`<a id="last" href="%s?page=%d" class="btn btn-outline-secondary">%s <i class="fa fa-forward"></i></a></div>`, base, pageCount, d.Locale.T("last"))
	out += fmt.Sprintf(`<script>var pagecount=%d,actualpage=%d;
if(actualpage==1){['first','prev'].forEach(function(id){var e=document.getElementById(id);if(e)e.className+=' disabled';});}
if(pagecount==actualpage){['last','next'].forEach(function(id){var e=document.getElementById(id);if(e)e.className+=' disabled';});}</script>`, pageCount, actualPage)
	return out
}

func (d *Renderer) GenerateLineChart(elementID, name string, data map[string]int) string {
	labels := make([]string, 0, len(data))
	values := make([]string, 0, len(data))
	for k, v := range data {
		labels = append(labels, k)
		values = append(values, fmt.Sprintf("%d", v))
	}
	return fmt.Sprintf(`<script>var ctx=document.getElementById("%s");new Chart(ctx,{type:'line',data:{labels:%q,datasets:[{label:'%s',data:[%s],backgroundColor:['rgba(255,99,132,0.2)'],borderColor:['rgba(255,99,132,1)']}]},options:{responsive:true,maintainAspectRatio:false,scales:{yAxes:[{ticks:{beginAtZero:true}}]}}});</script>`,
		elementID, strings.Join(labels, `","`), html.EscapeString(name), strings.Join(values, ","))
}

type urlValues map[string]string

func QueryValues(r *http.Request) urlValues {
	out := urlValues{}
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out
}

func PageNum(r *http.Request) (offset int, actual int) {
	actual = 1
	if p := r.URL.Query().Get("page"); p != "" {
		var n int
		if _, err := fmt.Sscanf(p, "%d", &n); err == nil && n > 0 {
			actual = n
		}
	}
	offset = (actual - 1) * 10
	return offset, actual
}
