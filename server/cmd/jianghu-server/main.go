package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/leiouliao-coder/jianghu/server/internal/config"
	"github.com/leiouliao-coder/jianghu/server/internal/gamedata"
	"github.com/leiouliao-coder/jianghu/server/internal/session"
	"github.com/leiouliao-coder/jianghu/server/internal/store"
	"github.com/leiouliao-coder/jianghu/server/internal/transport/httpapi"
	"github.com/leiouliao-coder/jianghu/server/internal/transport/ws"
	"github.com/leiouliao-coder/jianghu/server/internal/world"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var handler http.Handler = httpapi.NewRouter()
	var gateway *ws.Server
	if cfg.DatabaseURL != "" {
		// Fail fast: invalid data or an unreachable database must stop startup.
		data, err := gamedata.Load(cfg.DataDir)
		if err != nil {
			return fmt.Errorf("load %s: %w", cfg.DataDir, err)
		}
		st, err := store.Open(ctx, cfg.DatabaseURL, data)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer st.Close()
		if err := st.Migrate(ctx); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		log.Print("database ready, migrations applied")
		w := world.New(data)
		worldCtx, stopWorld := context.WithCancel(context.Background())
		defer stopWorld()
		go w.Run(worldCtx)
		gateway = ws.New(st, session.NewPresence(), w, ws.Config{
			Linger:           time.Duration(data.World.Combat.SafeZoneDisconnectSeconds) * time.Second,
			SessionTTL:       7 * 24 * time.Hour,
			IdleTimeout:      60 * time.Second,
			MaxLoginFailures: 5,
			Map:              world.ClientMap(data),
		})
		mux := http.NewServeMux()
		mux.Handle("/v1/ws", gateway)
		mux.Handle("/", handler)
		handler = mux
	} else {
		log.Print("JIANGHU_DATABASE_URL not set; running without database")
	}
	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 * 1024,
	}
	served := make(chan error, 1)
	go func() { served <- server.ListenAndServe() }()
	log.Printf("%s version=%s listening=%s websocket=%v", httpapi.Service, httpapi.Version, cfg.ListenAddr, gateway != nil)
	select {
	case err := <-served:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		stop() // A second termination signal may force immediate exit.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return err
		}
		if gateway != nil {
			if err := gateway.Shutdown(shutdownCtx); err != nil {
				log.Printf("websocket shutdown: %v", err)
			}
			// Lingering characters would leave after their timers; save them now.
			gateway.SavePositions(shutdownCtx)
		}
		err := <-served
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		log.Print("server shutdown complete")
		return nil
	}
}
