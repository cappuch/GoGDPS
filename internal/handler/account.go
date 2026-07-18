package handler

import (
	"net/http"

	"gogdps/internal/netutil"
	"gogdps/internal/service"
)

type AccountHandler struct {
	auth *service.AuthService
	save *service.AccountSaveService
}

func NewAccountHandler(auth *service.AuthService, save *service.AccountSaveService) *AccountHandler {
	return &AccountHandler{auth: auth, save: save}
}

func (h *AccountHandler) Register(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}
	if form["userName"] == "" {
		writeText(w, "-1")
		return
	}

	result, err := h.auth.Register(r.Context(), form["userName"], form["password"], form["email"])
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *AccountHandler) Login(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}

	ip := netutil.ClientIP(r)
	result, err := h.auth.Login(r.Context(), form["userName"], form["password"], form["gjp2"], form["udid"], ip)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *AccountHandler) Backup(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}
	result, err := h.save.Backup(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}

func (h *AccountHandler) Sync(w http.ResponseWriter, r *http.Request) {
	form, err := formValues(r)
	if err != nil {
		writeText(w, "-1")
		return
	}
	result, err := h.save.Sync(r.Context(), form)
	if err != nil {
		writeText(w, "-1")
		return
	}
	writeText(w, result)
}
