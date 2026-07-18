package app

import (
	"context"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"gogdps/internal/config"
	"gogdps/internal/captcha"
	"gogdps/internal/discord"
	"gogdps/internal/handler"
	"gogdps/internal/netutil"
	"gogdps/internal/service"
	"gogdps/internal/store"
)

type App struct {
	cfg    *config.Config
	server *http.Server
	store  *store.Store
}

type requestServices struct {
	auth     *service.AuthService
	gjp      *service.GJPService
	identity *service.IdentityService
	levels   *service.LevelsService
	scores   *service.ScoresService
	social   *service.SocialService
	comments *service.CommentsService
	commands *service.CommandsService
	likes    *service.LikesService
	save     *service.AccountSaveService
	daily    *service.DailyService
	rewards  *service.RewardsService
	songs    *service.SongsService
	packs    *service.PacksService
	profiles *service.ProfilesService
	messages *service.MessagesService
	mods     *service.ModService
}

func New(cfg *config.Config) (*App, error) {
	st, err := store.New(cfg)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok backend=" + st.Backend))
	})

	captchaValidator := captcha.NewValidator(&cfg.Security.Captcha)
	logsDir := filepath.Join(st.DataDir, "logs")
	dashboardDir, _ := filepath.Abs(cfg.Paths.DashboardDir)
	dc := discord.NewClient(&cfg.Discord)

	mux.Handle("/tools/stats/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := netutil.ClientIP(r)
		auth := service.NewAuthService(st, &cfg.Security, func() string { return ip })
		identity := service.NewIdentityService(st, &cfg.Security, service.NewGJPService(st, &cfg.Security, func() string { return ip }))
		statsSvc := service.NewStatsService(identity, auth)
		handler.NewStatsHandler(statsSvc).ServeHTTP(w, r)
	}))
	mux.Handle("/tools/bot/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := netutil.ClientIP(r)
		identity := service.NewIdentityService(st, &cfg.Security, service.NewGJPService(st, &cfg.Security, func() string { return ip }))
		songsSvc := service.NewSongsService(identity)
		botsSvc := service.NewBotsService(identity, dc, songsSvc, cfg)
		handler.NewBotsHandler(botsSvc).ServeHTTP(w, r)
	}))
	mux.Handle("/tools/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := netutil.ClientIP(r)
		auth := service.NewAuthService(st, &cfg.Security, func() string { return ip })
		identity := service.NewIdentityService(st, &cfg.Security, service.NewGJPService(st, &cfg.Security, func() string { return ip }))
		toolsSvc := service.NewToolsService(auth, identity, st, cfg.Reupload)
		cronSvc := service.NewCronService(identity, logsDir)
		songsSvc := service.NewSongsService(identity)
		localHost := r.Host
		if idx := strings.Index(localHost, ":"); idx >= 0 {
			localHost = localHost[:idx]
		}
		handler.NewToolsHandler(auth, captchaValidator, songsSvc, toolsSvc, cronSvc, localHost).ServeHTTP(w, r)
	}))
	mux.Handle("/tools", http.RedirectHandler("/tools/", http.StatusMovedPermanently))

	mux.Handle("/dashboard/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := netutil.ClientIP(r)
		auth := service.NewAuthService(st, &cfg.Security, func() string { return ip })
		identity := service.NewIdentityService(st, &cfg.Security, service.NewGJPService(st, &cfg.Security, func() string { return ip }))
		songsSvc := service.NewSongsService(identity)
		dashSvc := service.NewDashboardService(identity)
		handler.NewDashboardHandler(auth, identity, dashSvc, songsSvc, dashboardDir).ServeHTTP(w, r)
	}))
	mux.Handle("/dashboard", http.RedirectHandler("/dashboard/", http.StatusMovedPermanently))

	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := netutil.ClientIP(r)
		svcs := newRequestServices(st, cfg, ip)
		h := &handler.Handler{
			Account:  handler.NewAccountHandler(svcs.auth, svcs.save),
			Levels:   handler.NewLevelsHandler(svcs.identity, svcs.levels),
			Scores:   handler.NewScoresHandler(svcs.identity, svcs.scores),
			Social:   handler.NewSocialHandler(svcs.identity, svcs.social),
			Comments: handler.NewCommentsHandler(svcs.identity, svcs.comments),
			Packs:    handler.NewPacksHandler(svcs.identity, svcs.packs),
			Profiles: handler.NewProfilesHandler(svcs.identity, svcs.profiles),
			Messages: handler.NewMessagesHandler(svcs.identity, svcs.messages),
			Misc:     handler.NewMiscHandler(svcs.likes, svcs.songs, svcs.rewards, cfg.TopArtists.Redirect),
			Mods:     handler.NewModsHandler(svcs.identity, svcs.mods),
		}
		h.ServeHTTP(w, r)
	}))

	return &App{
		cfg:   cfg,
		store: st,
		server: &http.Server{
			Addr:    cfg.Server.Addr,
			Handler: mux,
		},
	}, nil
}

func newRequestServices(st *store.Store, cfg *config.Config, ip string) *requestServices {
	ipFn := func() string { return ip }
	gjp := service.NewGJPService(st, &cfg.Security, ipFn)
	identity := service.NewIdentityService(st, &cfg.Security, gjp)
	auth := service.NewAuthService(st, &cfg.Security, ipFn)
	daily := service.NewDailyService(st)
	dc := discord.NewClient(&cfg.Discord)
	commands := service.NewCommandsService(identity, st, dc)
	return &requestServices{
		auth:     auth,
		gjp:      gjp,
		identity: identity,
		levels:   service.NewLevelsService(st, identity, daily),
		scores:   service.NewScoresService(identity),
		social:   service.NewSocialService(identity),
		comments: service.NewCommentsService(identity, commands),
		commands: commands,
		likes:    service.NewLikesService(identity),
		save:     service.NewAccountSaveService(st, auth),
		daily:    daily,
		rewards:  service.NewRewardsService(identity, &cfg.Chests),
		songs:    service.NewSongsService(identity),
		packs:    service.NewPacksService(st, identity, ip),
		profiles: service.NewProfilesService(identity),
		messages: service.NewMessagesService(identity),
		mods:     service.NewModService(identity),
	}
}

func (a *App) Run() error {
	log.Printf("GoGDPS listening on %s (database: %s)", a.cfg.Server.Addr, a.store.Backend)
	return a.server.ListenAndServe()
}

func (a *App) Shutdown(ctx context.Context) error {
	return a.server.Shutdown(ctx)
}

func (a *App) Close() error {
	return a.store.Close()
}
