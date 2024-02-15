package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ardanlabs/conf/v3"
	goredislib "github.com/redis/go-redis/v9"
	"github.com/rschio/rinha/internal/core/client"
	"github.com/rschio/rinha/internal/core/client/store/clientdb"
	db "github.com/rschio/rinha/internal/data/dbsql/pgx"
	"github.com/rschio/rinha/internal/handlers"
	"github.com/rschio/rinha/internal/logger"
	"github.com/rschio/rinha/internal/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

var build = "develop"

func main() {
	log := logger.New("Rinha")

	if err := run(log); err != nil {
		log.Error("startup", "ERROR", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx := context.Background()

	// =========================================================================
	// Configuration

	cfg := struct {
		conf.Version
		Env string `conf:"default:DEV"`
		Web struct {
			Port            int           `conf:"default:8080"`
			ShutdownTimeout time.Duration `conf:"default:20s"`
		}
		DB struct {
			User       string `conf:"default:postgres"`
			Password   string `conf:"default:postgres,mask"`
			Host       string `conf:"default:0.0.0.0:5432"` // TODO: change to postgres
			Name       string `conf:"default:postgres"`
			DisableTLS bool   `conf:"default:true"`
		}
		Redis struct {
			Host string `conf:"default:0.0.0.0:6379"` // TODO: change to redis.
		}
		OTEL struct {
			Endpoint            string  `conf:"default:otel-collector:4317"`
			ServiceName         string  `conf:"default:Rinha"`
			TraceSampleFraction float64 `conf:"default:1.0"`
			EnableTrace         bool    `conf:"default:true"`
		}
	}{
		Version: conf.Version{
			Build: build,
		},
	}

	const prefix = "RINHA"
	help, err := conf.Parse(prefix, &cfg)
	if err != nil {
		if errors.Is(err, conf.ErrHelpWanted) {
			fmt.Println(help)
			return nil
		}
		return fmt.Errorf("parsing config: %w", err)
	}

	// =========================================================================
	// App Starting

	log.Info("starting service", "version", build)
	defer log.Info("shutdown complete")

	out, err := conf.String(&cfg)
	if err != nil {
		return fmt.Errorf("generating config for output: %w", err)
	}
	log.Info("startup", "config", out)

	// =========================================================================
	// Trace support

	tracerProvider, err := trace.NewProvider(ctx, trace.Config{
		Env:            cfg.Env,
		Endpoint:       cfg.OTEL.Endpoint,
		Service:        cfg.OTEL.ServiceName,
		SampleFraction: cfg.OTEL.TraceSampleFraction,
		DiscardTraces:  !cfg.OTEL.EnableTrace,
	})
	if err != nil {
		return fmt.Errorf("constructing tracer provider: %w", err)
	}
	defer tracerProvider.Shutdown(ctx)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	tracer := otel.GetTracerProvider().Tracer("service")

	// =========================================================================
	// Database Support

	log.Info("startup", "status", "initializing database support", "host", cfg.DB.Host)

	dbCfg := db.Config{
		User:       cfg.DB.User,
		Password:   cfg.DB.Password,
		Host:       cfg.DB.Host,
		Name:       cfg.DB.Name,
		DisableTLS: cfg.DB.DisableTLS,
	}
	database, err := db.Open(ctx, dbCfg)
	if err != nil {
		return fmt.Errorf("connecting to db: %w", err)
	}
	defer func() {
		log.Info("shutdown", "status", "stopping database support", "host", cfg.DB.Host)
		database.Close()
	}()

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := db.StatusCheck(ctxWithTimeout, database); err != nil {
		return fmt.Errorf("database not health: %w", err)
	}

	// =========================================================================
	// Start Redis

	redis := goredislib.NewClient(&goredislib.Options{
		Addr: cfg.Redis.Host,
	})

	// =========================================================================
	// Start API Service

	log.Info("startup", "status", "initializing RINHA API support")

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	core := client.NewCore(clientdb.NewStore(log, database), redis)
	srv := handlers.NewServer(log, core)
	mux := handlers.APIMux(srv, tracer)

	api := http.Server{
		Addr:     fmt.Sprintf(":%d", cfg.Web.Port),
		Handler:  mux,
		ErrorLog: slog.NewLogLogger(log.Handler(), slog.LevelInfo),
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Info("startup", "status", "api router started", "host", api.Addr)
		serverErrors <- api.ListenAndServe()
	}()

	// =========================================================================
	// Shutdown

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		log.Info("shutdown", "status", "shutdown started", "signal", sig)
		defer log.Info("shutdown", "status", "shutdown complete", "signal", sig)

		ctx, cancel := context.WithTimeout(context.Background(), cfg.Web.ShutdownTimeout)
		defer cancel()

		if err := api.Shutdown(ctx); err != nil {
			api.Close()
			return fmt.Errorf("could not stop server gracefully: %w", err)
		}
	}

	return nil
}
