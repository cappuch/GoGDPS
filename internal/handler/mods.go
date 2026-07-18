package handler

import (
	"net/http"
	"strconv"

	"gogdps/internal/service"
)

type ModsHandler struct {
	identity *service.IdentityService
	mods     *service.ModService
}

func NewModsHandler(identity *service.IdentityService, mods *service.ModService) *ModsHandler {
	return &ModsHandler{identity: identity, mods: mods}
}

func (h *ModsHandler) RateStars(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}
	accountID, err := h.identity.RequireAccountFromForm(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	accInt, _ := strconv.Atoi(accountID)
	result, err := h.mods.RateStars(r.Context(), accInt, form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *ModsHandler) SuggestStars(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}
	accountID, err := h.identity.RequireAccountFromForm(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	accInt, _ := strconv.Atoi(accountID)
	result, err := h.mods.SuggestStars(r.Context(), accInt, form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *ModsHandler) RateDemon(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}
	if form["rating"] == "" || form["levelID"] == "" || form["accountID"] == "" {
		writeText(w, "-1")
		return
	}
	accountID, err := h.identity.RequireAccountFromForm(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	accInt, _ := strconv.Atoi(accountID)
	result, err := h.mods.RateDemon(r.Context(), accInt, form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *ModsHandler) RequestUserAccess(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		return
	}
	accountID, err := h.identity.RequireAccountFromForm(r.Context(), form)
	if err != nil {
		return
	}
	accInt, _ := strconv.Atoi(accountID)
	result, err := h.mods.RequestUserAccess(r.Context(), accInt)
	if err != nil {
		return
	}
	writeText(w, result)
}
