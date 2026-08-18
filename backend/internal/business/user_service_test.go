package business_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/siamionv/finy/internal/business"
	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/pkg/cerr"
)

// fakeUserReader answers with whatever the case under test needs, and records the
// id it was handed.
type fakeUserReader struct {
	user *entity.UserDB
	err  error
	seen int
}

func (f *fakeUserReader) GetUserByID(_ context.Context, id int) (*entity.UserDB, error) {
	f.seen = id

	if f.err != nil {
		return nil, f.err
	}

	return f.user, nil
}

func TestUserServiceGetUserDropsPasswordHash(t *testing.T) {
	iconURL := "https://finy.by/icons/default.png"
	createdAt := time.Date(2026, time.August, 18, 10, 0, 0, 0, time.UTC)

	repo := &fakeUserReader{user: &entity.UserDB{
		ID:           42,
		Username:     "johndoe",
		PasswordHash: "must-not-escape",
		IconURL:      &iconURL,
		CreatedAt:    createdAt,
	}}

	user, err := business.NewUserService(repo).GetUser(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.seen != 42 {
		t.Errorf("repo saw id %d, want 42", repo.seen)
	}

	want := entity.User{ID: 42, Username: "johndoe", IconURL: &iconURL, CreatedAt: createdAt}
	if *user != want {
		t.Errorf("got %+v, want %+v", *user, want)
	}
}

func TestUserServiceGetUserNotFoundPassesThrough(t *testing.T) {
	repo := &fakeUserReader{err: entity.ErrUserNotFound}

	_, err := business.NewUserService(repo).GetUser(context.Background(), 7)
	if !errors.Is(err, entity.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserServiceGetUserWrapsInternalError(t *testing.T) {
	cause := cerr.New("connection refused", cerr.Internal)
	repo := &fakeUserReader{err: cause}

	_, err := business.NewUserService(repo).GetUser(context.Background(), 7)
	if !errors.Is(err, cerr.Internal) {
		t.Fatalf("expected the error to stay Internal, got %v", err)
	}
	if !errors.Is(err, cause) {
		t.Errorf("expected the cause to be carried, got %v", err)
	}
}
