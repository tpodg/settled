package app

import (
	"log/slog"
	"testing"

	"github.com/tpodg/settled/internal/config"
)

func TestNew(t *testing.T) {
	cfg := &config.Config{
		Servers: []config.ServerConfig{
			{Name: "test", Address: "1.2.3.4", User: "admin"},
		},
	}

	a := New(cfg)

	if a == nil {
		t.Fatal("expected App instance, got nil")
	}

	if a.Config != cfg {
		t.Error("expected App to have the provided config")
	}

	if a.Logger == nil {
		t.Error("expected App to have a logger")
	}

	// Simple check if it's the right logger type
	if _, ok := any(a.Logger).(*slog.Logger); !ok {
		t.Error("expected Logger to be of type *slog.Logger")
	}
}
