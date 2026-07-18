package handler

import (
	"net/http"
	"strconv"

	"gogdps/internal/netutil"
	"gogdps/internal/sanitize"
	"gogdps/internal/service"
)

type PacksHandler struct {
	identity *service.IdentityService
	packs    *service.PacksService
}

func NewPacksHandler(identity *service.IdentityService, packs *service.PacksService) *PacksHandler {
	return &PacksHandler{identity: identity, packs: packs}
}

func (h *PacksHandler) GetMapPacks(w http.ResponseWriter, r *http.Request) {
	form, _ := formValues(r)
	page, _ := strconv.Atoi(sanitize.Number(form["page"]))
	result, err := h.packs.GetMapPacks(r.Context(), page)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *PacksHandler) GetGauntlets(w http.ResponseWriter, r *http.Request) {
	result, err := h.packs.GetGauntlets(r.Context())
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *PacksHandler) GetLevelLists(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}
	result, err := h.packs.GetLevelLists(r.Context(), form)
	if err != nil || result == "-1" {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *PacksHandler) UploadLevelList(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}
	accountID, err := h.identity.RequireAccountFromForm(r.Context(), form)
	if err != nil {
		writeText(w, "-9")
		return
	}
	accInt, _ := strconv.Atoi(accountID)
	result, err := h.packs.UploadLevelList(r.Context(), accInt, form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *PacksHandler) DeleteLevelList(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.packs.DeleteLevelList(r.Context(), accInt, form["listID"])
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

type ProfilesHandler struct {
	identity *service.IdentityService
	profiles *service.ProfilesService
}

func NewProfilesHandler(identity *service.IdentityService, profiles *service.ProfilesService) *ProfilesHandler {
	return &ProfilesHandler{identity: identity, profiles: profiles}
}

func (h *ProfilesHandler) GetUserInfo(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}
	result, err := h.profiles.GetUserInfo(r.Context(), form)
	if err != nil || result == "-1" {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *ProfilesHandler) GetUsers(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}
	result, err := h.profiles.GetUsers(r.Context(), form)
	if err != nil || result == "-1" {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *ProfilesHandler) UpdateAccSettings(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.profiles.UpdateAccSettings(r.Context(), accInt, form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

type MessagesHandler struct {
	identity *service.IdentityService
	messages *service.MessagesService
}

func NewMessagesHandler(identity *service.IdentityService, messages *service.MessagesService) *MessagesHandler {
	return &MessagesHandler{identity: identity, messages: messages}
}

func (h *MessagesHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.messages.GetMessages(r.Context(), accInt, form)
	if result == "-2" {
		writeText(w, "-2")
		return
	}
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *MessagesHandler) UploadMessage(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.messages.UploadMessage(r.Context(), accInt, form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *MessagesHandler) DownloadMessage(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.messages.DownloadMessage(r.Context(), accInt, form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *MessagesHandler) DeleteMessages(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.messages.DeleteMessages(r.Context(), accInt, form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *LevelsHandler) Report(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil || form["levelID"] == "" {
		writeText(w, "-1")
		return
	}
	ip := netutil.ClientIP(r)
	result, err := h.levels.Report(r.Context(), sanitize.Remove(form["levelID"]), ip)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}
