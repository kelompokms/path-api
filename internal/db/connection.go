package db

import (
	"context"
	"database/sql"
	"embed"
	"path-api/internal/utils"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func Connect(ctx context.Context) (*Queries, *pgxpool.Pool, *sql.DB, error) {
	config, err := pgxpool.ParseConfig(utils.GetDatabaseURL())
	if err != nil {
		return nil, nil, nil, err
	}

	// Align TIMESTAMP WITHOUT TIME ZONE decoding with the database's active
	// TimeZone so timestamps are always interpreted accurately before converting
	// responses to GMT+7.
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		var tzName string
		if err := conn.QueryRow(ctx, "SHOW TIMEZONE").Scan(&tzName); err == nil {
			conn.TypeMap().RegisterType(&pgtype.Type{
				Name:  "timestamp",
				OID:   pgtype.TimestampOID,
				Codec: &pgtype.TimestampCodec{ScanLocation: utils.ResolvePostgresLocation(tzName)},
			})
		}
		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, nil, nil, err
	}

	dbConn := stdlib.OpenDBFromPool(pool)

	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return nil, nil, nil, err
	}
	if err := goose.Up(dbConn, "migrations"); err != nil {
		return nil, nil, nil, err
	}

	queries := New(pool)

	return queries, pool, dbConn, nil
}
