package handler

import (
	"net/http"

	"gogdps/internal/netutil"
	"gogdps/internal/sanitize"
	"gogdps/internal/service"
)

type MiscHandler struct {
	likes   *service.LikesService
	songs   *service.SongsService
	rewards *service.RewardsService
	redirectTopArtists bool
}

func NewMiscHandler(likes *service.LikesService, songs *service.SongsService, rewards *service.RewardsService, redirectTopArtists bool) *MiscHandler {
	return &MiscHandler{likes: likes, songs: songs, rewards: rewards, redirectTopArtists: redirectTopArtists}
}

func (h *MiscHandler) CustomContentURL(w http.ResponseWriter, r *http.Request) {
	writeText(w, "https://geometrydashfiles.b-cdn.net")
}

func (h *MiscHandler) AccountURL(w http.ResponseWriter, r *http.Request) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	path := r.URL.Path
	if idx := len(path) - len("/getAccountURL.php"); idx > 0 && path[idx:] == "/getAccountURL.php" {
		path = path[:idx]
	} else if idx := len(path) - len("/getAccountURL"); idx > 0 && path[idx:] == "/getAccountURL" {
		path = path[:idx]
	} else {
		path = ""
	}
	writeText(w, scheme+"://"+host+path)
}

func (h *MiscHandler) LikeItem(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}
	ip := netutil.ClientIP(r)
	result, err := h.likes.LikeItem(r.Context(), ip, form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *MiscHandler) GetSongInfo(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil || form["songID"] == "" {
		writeText(w, "-1")
		return
	}
	result, err := h.songs.GetSongInfo(r.Context(), form["songID"])
	if err != nil {
		writeText(w, "-1")
		return
	}
	if result == "-2" {
		writeText(w, "-2")
		return
	}
	writeText(w, result)
}

func (h *MiscHandler) GetTopArtists(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}
	page := atoiDefault(sanitize.Number(form["page"]), 0)
	result, err := h.songs.GetTopArtists(r.Context(), page, h.redirectTopArtists)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *MiscHandler) GetRewards(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}
	result, err := h.rewards.GetRewards(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *MiscHandler) GetChallenges(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}
	result, err := h.rewards.GetChallenges(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}
