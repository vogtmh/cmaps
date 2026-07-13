// Command cmaps is the CompanyMaps server: it wires the configuration, the
// bolt store and the domain services into the web layer, starts the
// background schedulers and serves HTTP until it receives SIGINT/SIGTERM,
// then shuts down gracefully.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"companymaps/internal/config"
	"companymaps/internal/directory"
	"companymaps/internal/integrations/geo"
	"companymaps/internal/integrations/robin"
	"companymaps/internal/store"
	"companymaps/internal/web"
)

func main() {
	cfg, err := config.LoadOrCreate()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Ensure the data directory and its subfolders exist.
	for _, d := range []string{cfg.DataDir, cfg.DataPath("maps"), cfg.DataPath("avatarcache"), cfg.DataPath("logos"), cfg.DataPath("itemtypes")} {
		if err := os.MkdirAll(d, 0755); err != nil {
			log.Fatalf("creating data dir %s: %v", d, err)
		}
	}

	db, err := store.Open(cfg.DataPath("cmaps.db"))
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	// Backfill newer optional settings so they appear in the admin panel on
	// installations created before the setting existed.
	for name, def := range map[string]string{"reportURL": "", "nomapText": "", "nomapLink": ""} {
		if err := db.EnsureSetting(name, def); err != nil {
			log.Fatalf("ensure settings: %v", err)
		}
	}

	// Domain services.
	dirSvc := &directory.Syncer{DB: db, AvatarDir: cfg.DataPath("avatarcache")}
	robinSvc := &robin.Service{DB: db}
	geoSvc := &geo.Service{DB: db}

	srv, err := web.NewServer(cfg, db, dirSvc, robinSvc, geoSvc)
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	// Migrate the legacy single EntraID connection into the multi-source model.
	dirSvc.MigrateEntraConfig()

	// Background schedulers (directory syncs, Robin, health checks, janitor).
	srv.StartSchedulers()

	httpSrv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: srv.Handler(),
	}

	// Graceful shutdown: stop accepting connections on SIGINT/SIGTERM, give
	// in-flight requests a grace period, then close the bolt store (via the
	// deferred db.Close) so the file is never torn mid-write.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("CompanyMaps 9 listening on %s (data dir: %s)", cfg.ListenAddr, cfg.DataDir)
		errCh <- httpSrv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	case <-ctx.Done():
		log.Printf("shutdown signal received, draining connections…")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
		log.Printf("server stopped")
	}
}
