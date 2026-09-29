package server

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

	"bmj-backend/internal/config"
	"bmj-backend/internal/middleware"
	"bmj-backend/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

const shutdownTimeout = 15 * time.Second

// Server is the HTTP API process.
type Server struct {
	httpServer *http.Server
	engine     *gin.Engine
}

// New builds the router and HTTP server.
// pool may be nil when the database is not configured. Process health checks do not use it.
func New(cfg config.Config, pool *pgxpool.Pool) (*Server, error) {
	applyGinMode(cfg.AppEnv)

	engine := gin.New()
	if err := engine.SetTrustedProxies(nil); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}

	engine.Use(recovery(), gin.Logger(), middleware.CORS(cfg.CORSOrigins))
	registerRoutes(engine, pool, cfg.JWTSecret, storage.New(cfg))

	return &Server{
		engine: engine,
		httpServer: &http.Server{
			Addr:              ":" + cfg.AppPort,
			Handler:           engine,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}, nil
}

// Handler returns the router for tests and in-process use.
func (s *Server) Handler() http.Handler {
	return s.engine
}

// Run listens until the context is canceled or SIGINT/SIGTERM is received,
// then waits for in-flight requests to finish.
func (s *Server) Run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("http server listening on %s", s.httpServer.Addr)
		err := s.httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		log.Print("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}

	return nil
}

func applyGinMode(appEnv string) {
	if appEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
		return
	}
	if gin.Mode() == gin.TestMode {
		return
	}
	gin.SetMode(gin.DebugMode)
}
