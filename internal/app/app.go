package app

import (
	"log/slog"
	"os"

	"github.com/tpodg/settled/internal/config"
)

type App struct {
	Logger *slog.Logger
	Config *config.Config
}

func New(cfg *config.Config) *App {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	return &App{
		Logger: logger,
		Config: cfg,
	}
}
