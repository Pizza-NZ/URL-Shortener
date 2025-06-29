package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/tern/v2/migrate"
	"github.com/pizza-nz/url-shortener/types"
)

func Migration(conn string) error {
	ctx := context.Background()

	pgx, err := pgx.Connect(ctx, conn)
	if err != nil {
		return types.NewDBError("Migration failed to pgx connect to DB", err)
	}
	if err := pgx.Ping(ctx); err != nil {
		return types.NewDBError("Migration failed to ping to DB", err)
	}

	m, _ := migrate.NewMigrator(ctx, pgx, "my_schema_version")

	m.Migrations = []*migrate.Migration{
		{
			Sequence: 1,
			Name:     "1",
			UpSQL:    `CREATE TABLE table_urls (short_url text primary key, long_url text not null)`,
			DownSQL:  `DROP TABLE table_urls`,
		},
		{
			Sequence: 2,
			Name:     "2",
			UpSQL:    `CREATE TABLE table_counter (id SERIAL primary key, created_at TIMESTAMPTZ); INSERT INTO table_counter (created_at) VALUES (NOW())`,
			DownSQL:  `DROP TABLE table_counter`,
		},
	}

	m.MigrateTo(context.Background(), 2)

	return m.Migrate(ctx)
}
