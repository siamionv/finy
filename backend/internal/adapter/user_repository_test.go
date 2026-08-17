package adapter

import (
	"strings"
	"testing"

	"github.com/siamionv/finy/internal/entity"
)

func TestBuildInsertUserQuery(t *testing.T) {
	dto := entity.CreateUser{
		Username:     "john",
		PasswordHash: "hashed-password",
	}

	sql, args, err := buildInsertUserQuery(dto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(sql, "$1") {
		t.Errorf("expected query to contain $1 placeholder, got: %s", sql)
	}
	if strings.Contains(sql, "?") {
		t.Errorf("expected query to contain no ? placeholders, got: %s", sql)
	}

	wantArgs := []any{dto.Username, dto.PasswordHash}
	if len(args) != len(wantArgs) {
		t.Fatalf("expected %d args, got %d: %v", len(wantArgs), len(args), args)
	}
	for i, want := range wantArgs {
		if args[i] != want {
			t.Errorf("arg[%d] = %v, want %v", i, args[i], want)
		}
	}
}

func TestBuildGetUserByUsernameQuery(t *testing.T) {
	username := "john"

	sql, args, err := buildGetUserByUsernameQuery(username)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Regression: this query previously used the default '?' placeholder
	// format instead of Postgres-style '$N', which Postgres rejects
	// with a 42601 syntax error on every login attempt.
	if !strings.Contains(sql, "$1") {
		t.Errorf("expected query to contain $1 placeholder, got: %s", sql)
	}
	if strings.Contains(sql, "?") {
		t.Errorf("expected query to contain no ? placeholders, got: %s", sql)
	}

	wantArgs := []any{username}
	if len(args) != len(wantArgs) {
		t.Fatalf("expected %d args, got %d: %v", len(wantArgs), len(args), args)
	}
	for i, want := range wantArgs {
		if args[i] != want {
			t.Errorf("arg[%d] = %v, want %v", i, args[i], want)
		}
	}
}
