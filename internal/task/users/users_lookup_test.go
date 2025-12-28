package users

import (
	"context"
	"errors"
	"testing"
)

type stubServer struct {
	output string
	err    error
}

func (s *stubServer) ID() string      { return "stub" }
func (s *stubServer) Address() string { return "stub" }
func (s *stubServer) Execute(ctx context.Context, command string) (string, error) {
	return s.output, s.err
}

func TestLookupUserMissing(t *testing.T) {
	srv := &stubServer{output: missingUserSentinel}

	entry, err := lookupUser(context.Background(), srv, "alice")
	if err != nil {
		t.Fatalf("lookupUser returned error: %v", err)
	}
	if entry != nil {
		t.Fatalf("expected nil entry, got %+v", entry)
	}
}

func TestLookupUserPresent(t *testing.T) {
	srv := &stubServer{output: "alice:x:1000:1000::/home/alice:/bin/bash\n"}

	entry, err := lookupUser(context.Background(), srv, "alice")
	if err != nil {
		t.Fatalf("lookupUser returned error: %v", err)
	}
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if entry.home != "/home/alice" {
		t.Fatalf("expected home /home/alice, got %q", entry.home)
	}
}

func TestLookupUserError(t *testing.T) {
	expected := errors.New("execute failed")
	srv := &stubServer{err: expected}

	_, err := lookupUser(context.Background(), srv, "alice")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, expected) {
		t.Fatalf("expected error %v, got %v", expected, err)
	}
}
