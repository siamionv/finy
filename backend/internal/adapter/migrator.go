package adapter

import (
	"database/sql"

	"github.com/siamionv/finy/pkg/cerr"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver goose runs through
	"github.com/pressly/goose/v3"
)

// RunMigrations applies every pending migration in migrationsPath. It opens its
// own database/sql connection: goose cannot drive a pgx pool.
func RunMigrations(dsn, migrationsPath string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return cerr.New("failed to open sql connection", err, cerr.Internal).Loc().Time()
	}
	defer func() { _ = db.Close() }()

	if err := goose.Up(db, migrationsPath); err != nil {
		return cerr.New("failed to run migration", err, cerr.Internal).
			Loc().
			Time().
			With("path", migrationsPath)
	}

	return nil
}
