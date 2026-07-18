package handler

import (
	"net/http"
	"strconv"
	"strings"

	"gogdps/internal/service"
)

type BotsHandler struct {
	bots *service.BotsService
}

func NewBotsHandler(bots *service.BotsService) *BotsHandler {
	return &BotsHandler{bots: bots}
}

func (h *BotsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/tools/bot/")
	path = strings.TrimSuffix(path, ".php")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	var (
		out string
		err error
	)
	q := r.URL.Query()
	switch path {
	case "whoRatedBot":
		out, err = h.bots.WhoRated(r.Context(), q.Get("level"))
	case "userLevelSearchBot":
		_ = r.ParseForm()
		out, err = h.bots.UserLevelSearch(r.Context(), r.FormValue("str"))
	case "songSearchBot":
		out, err = h.bots.SongSearch(r.Context(), q.Get("str"))
	case "songListBot":
		page, _ := strconv.Atoi(q.Get("page"))
		out, err = h.bots.SongList(r.Context(), page)
	case "songAddBot":
		out, err = h.bots.SongAdd(r.Context(), q.Get("link"), q.Get("name"), q.Get("author"))
	case "playerStatsBot":
		out, err = h.bots.PlayerStats(r.Context(), q.Get("player"))
	case "modActionsBot":
		out, err = h.bots.ModActions(r.Context())
	case "levelSearchBot":
		out, err = h.bots.LevelSearch(r.Context(), q.Get("str"))
	case "leaderboardsBot":
		page, _ := strconv.Atoi(q.Get("page"))
		out, err = h.bots.Leaderboards(r.Context(), q.Get("type"), page)
	case "latestSongBot":
		out, err = h.bots.LatestSong(r.Context())
	case "discordLinkUnlink":
		out, err = h.bots.DiscordLinkUnlink(r.Context(), q.Get("secret"), q.Get("discordID"))
	case "discordLinkTransferRoles":
		out, err = h.bots.DiscordLinkTransferRoles(r.Context(), q.Get("secret"), q.Get("discordID"), q.Get("roles"))
	case "discordLinkResetPass":
		out, err = h.bots.DiscordLinkResetPass(r.Context(), q.Get("secret"), q.Get("discordID"))
	case "discordLinkReq":
		out, err = h.bots.DiscordLinkReq(r.Context(), q.Get("secret"), q.Get("discordID"), q.Get("account"))
	case "dailyLevelBot":
		out, err = h.bots.DailyLevel(r.Context())
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
