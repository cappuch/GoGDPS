package handler

import (
	"net/http"
	"strings"

	"gogdps/internal/service"
)

type StatsHandler struct {
	stats *service.StatsService
}

func NewStatsHandler(stats *service.StatsService) *StatsHandler {
	return &StatsHandler{stats: stats}
}

func (h *StatsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/tools/stats/")
	path = strings.TrimSuffix(path, ".php")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	var (
		out string
		err error
	)
	switch path {
	case "stats":
		out, err = h.stats.StatsPage(r.Context())
	case "vipList":
		out, err = h.stats.VIPList(r.Context())
	case "top24h":
		out, err = h.stats.Top24h(r.Context())
	case "songList":
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
		}
		out, err = h.stats.SongList(r.Context(), r.FormValue("name"), r.FormValue("type"))
	case "reportList":
		out, err = h.stats.ReportList(r.Context())
	case "packTable":
		out, err = h.stats.PackTable(r.Context())
	case "noLogIn":
		out, err = h.stats.NoLogIn(r.Context())
	case "dailyTable":
		out, err = h.stats.DailyTable(r.Context())
	case "modActions":
		out, err = h.stats.ModActions(r.Context())
	case "unlisted":
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
		}
		out, err = h.stats.Unlisted(r.Context(), r.FormValue("userName"), r.FormValue("password"))
	case "suggestList":
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
		}
		out, err = h.stats.SuggestList(r.Context(), r.FormValue("userName"), r.FormValue("password"))
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write([]byte(out))
}
