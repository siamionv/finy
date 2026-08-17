package adapter

import (
	"context"
	"errors"
	"fmt"

	"github.com/siamionv/finy/internal/entity"
	"github.com/siamionv/finy/pkg/cerr"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const pgUniqueViolationCode = "23505"

const (
	usersTable           = "users"
	usersColID           = "id"
	usersColUsername     = "username"
	usersColIconURL      = "icon_url"
	usersColPasswordHash = "password_hash"
	usersColCreatedAt    = "created_at"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) InsertUser(
	ctx context.Context,
	dto entity.CreateUser,
) (*entity.User, error) {
	sql, args, err := sq.Insert(usersTable).
		PlaceholderFormat(sq.Dollar).
		Columns(usersColUsername, usersColPasswordHash).
		Values(dto.Username, dto.PasswordHash).
		Suffix(fmt.Sprintf("RETURNING %s, %s, %s, %s", usersColID, usersColUsername, usersColIconURL, usersColCreatedAt)).
		ToSql()
	if err != nil {
		// Internal, not Invalid: the query is built from constants and a struct
		// we control, so a failure here is our bug, never the caller's input.
		return nil, cerr.New("failed to build sql query", err, cerr.Internal).
			Loc().
			Time().
			With("table", usersTable)
	}

	var user entity.User
	if err := r.db.QueryRow(ctx, sql, args...).
		Scan(&user.ID, &user.Username, &user.IconURL, &user.CreatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolationCode {
			return nil, entity.ErrUserAlreadyExist.Loc().Time().With("username", dto.Username)
		}

		return nil, cerr.New("failed to insert user", err, cerr.Internal).
			Loc().
			Time().
			With("table", usersTable, "username", dto.Username)
	}

	return &user, nil
}
