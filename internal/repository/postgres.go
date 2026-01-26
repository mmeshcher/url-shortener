package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
	sb   squirrel.StatementBuilderType
}

func NewPostgresRepository(dsn string) (*PostgresRepository, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := runMigrations(dsn); err != nil {
		return nil, fmt.Errorf("migrations failed: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	log.Println("PostgreSQL repository initialized successfully")

	return &PostgresRepository{pool: pool,
		sb: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}, nil
}

func runMigrations(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open database for migrations: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping database for migrations: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		"postgres", driver,
	)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("apply migrations: %w", err)
	}

	log.Println("Migrations applied successfully")
	return nil
}

func (p *PostgresRepository) SaveURL(ctx context.Context, shortID, originalURL string) (bool, error) {
	query, args, err := p.sb.
		Insert("urls").
		Columns("short_id", "original_url").
		Values(shortID, originalURL).
		Suffix("ON CONFLICT (original_url) DO NOTHING").
		ToSql()
	if err != nil {
		return false, fmt.Errorf("build query: %w", err)
	}

	cmdTag, err := p.pool.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return true, nil
		}
		return false, fmt.Errorf("execute query: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return true, nil
	}

	return false, nil
}

func (p *PostgresRepository) GetShortIDForOriginalURL(ctx context.Context, originalURL string) (string, error) {
	return p.GetShortID(ctx, originalURL)
}

func (p *PostgresRepository) GetOriginalURL(ctx context.Context, shortID string) (string, error) {
	query, args, err := p.sb.
		Select("original_url").
		From("urls").
		Where(squirrel.Eq{"short_id": shortID}).
		ToSql()

	if err != nil {
		return "", fmt.Errorf("build query: %w", err)
	}

	var originalURL string
	err = p.pool.QueryRow(ctx, query, args...).Scan(&originalURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("not found: %w", err)
		}
		return "", fmt.Errorf("query row: %w", err)
	}

	return originalURL, nil
}

func (p *PostgresRepository) GetShortID(ctx context.Context, originalURL string) (string, error) {
	query, args, err := p.sb.
		Select("short_id").
		From("urls").
		Where(squirrel.Eq{"original_url": originalURL}).
		ToSql()
	if err != nil {
		return "", fmt.Errorf("build query: %w", err)
	}

	var shortID string
	err = p.pool.QueryRow(ctx, query, args...).Scan(&shortID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("not found: %w", err)
		}
		return "", fmt.Errorf("query row: %w", err)
	}

	return shortID, nil
}

func (p *PostgresRepository) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

func (p *PostgresRepository) Close() error {
	p.pool.Close()
	return nil
}

func (p *PostgresRepository) ProcessURLBatch(ctx context.Context, batch []BatchItem) (map[string]string, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	originalURLs := make([]string, len(batch))
	for i, item := range batch {
		originalURLs[i] = item.OriginalURL
	}

	existingURLs, err := p.getExistingURLsInTransaction(ctx, tx, originalURLs)
	if err != nil {
		return nil, fmt.Errorf("get existing urls: %w", err)
	}

	result := make(map[string]string)
	urlsToInsert := make([]BatchItem, 0)

	for _, item := range batch {
		if shortID, exists := existingURLs[item.OriginalURL]; exists {
			result[item.OriginalURL] = shortID
		} else {
			urlsToInsert = append(urlsToInsert, item)
			result[item.OriginalURL] = item.ShortID
		}
	}

	if len(urlsToInsert) > 0 {
		if err := p.insertURLsInTransaction(ctx, tx, urlsToInsert); err != nil {
			return nil, fmt.Errorf("insert urls: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return result, nil
}

func (p *PostgresRepository) getExistingURLsInTransaction(ctx context.Context, tx pgx.Tx, originalURLs []string) (map[string]string, error) {
	if len(originalURLs) == 0 {
		return make(map[string]string), nil
	}

	query, args, err := p.sb.
		Select("short_id", "original_url").
		From("urls").
		Where(squirrel.Eq{"original_url": originalURLs}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query existing urls: %w", err)
	}
	defer rows.Close()

	existing := make(map[string]string)
	for rows.Next() {
		var shortID, originalURL string
		if err := rows.Scan(&shortID, &originalURL); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		existing[originalURL] = shortID
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return existing, nil
}

func (p *PostgresRepository) insertURLsInTransaction(ctx context.Context, tx pgx.Tx, urls []BatchItem) error {
	if len(urls) == 0 {
		return nil
	}

	insertBuilder := p.sb.
		Insert("urls").
		Columns("short_id", "original_url")
	for _, item := range urls {
		insertBuilder = insertBuilder.Values(item.ShortID, item.OriginalURL)
	}

	query, args, err := insertBuilder.Suffix("ON CONFLICT (original_url) DO NOTHING").ToSql()
	if err != nil {
		return fmt.Errorf("build insert query: %w", err)
	}

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("execute insert: %w", err)
	}

	return nil
}

type BatchItem struct {
	ShortID     string
	OriginalURL string
}
