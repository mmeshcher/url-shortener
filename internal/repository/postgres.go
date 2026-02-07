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
	"github.com/mmeshcher/url-shortener/internal/models"
)

type PostgresRepository struct {
	pool    *pgxpool.Pool
	sb      squirrel.StatementBuilderType
	baseURL string
}

func NewPostgresRepository(dsn, baseURL string) (*PostgresRepository, error) {
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

	return &PostgresRepository{
		pool:    pool,
		sb:      squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
		baseURL: baseURL,
	}, nil
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
		if err.Error() == "Dirty database version 1. Fix and force version." {
			log.Println("Database is in dirty state, forcing clean state...")
			if forceErr := m.Force(1); forceErr != nil {
				log.Printf("Failed to force version: %v", forceErr)
			}
			if retryErr := m.Up(); retryErr != nil && retryErr != migrate.ErrNoChange {
				return fmt.Errorf("apply migrations after force: %w", retryErr)
			}
		} else {
			return fmt.Errorf("apply migrations: %w", err)
		}
	}

	log.Println("Migrations applied successfully")
	return nil
}

func (p *PostgresRepository) SaveURL(ctx context.Context, shortID, originalURL, userID string) (string, bool, error) {
	query, args, err := p.sb.
		Insert("urls").
		Columns("short_id", "original_url", "user_id").
		Values(shortID, originalURL, userID).
		Suffix("ON CONFLICT (original_url) DO NOTHING").
		ToSql()
	if err != nil {
		return "", false, fmt.Errorf("build query: %w", err)
	}

	cmdTag, err := p.pool.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return "", true, nil
		}
		return "", false, fmt.Errorf("execute query: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		existingID, err := p.GetShortID(ctx, originalURL)
		if err != nil {
			return "", true, err
		}
		return existingID, true, nil
	}

	return shortID, false, nil
}

func (p *PostgresRepository) GetUserURLs(ctx context.Context, userID string) ([]models.UserURL, error) {
	query, args, err := p.sb.
		Select("short_id", "original_url").
		From("urls").
		Where(squirrel.And{
			squirrel.Eq{"user_id": userID},
			squirrel.Eq{"is_deleted": false},
		}).
		OrderBy("created_at DESC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query user URLs: %w", err)
	}
	defer rows.Close()

	var userURLs []models.UserURL
	for rows.Next() {
		var shortID, originalURL string
		if err := rows.Scan(&shortID, &originalURL); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		shortURL := fmt.Sprintf("%s/%s", p.baseURL, shortID)
		userURLs = append(userURLs, models.UserURL{
			ShortURL:    shortURL,
			OriginalURL: originalURL,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return userURLs, nil
}

func (p *PostgresRepository) GetShortIDForOriginalURL(ctx context.Context, originalURL string) (string, error) {
	return p.GetShortID(ctx, originalURL)
}

func (p *PostgresRepository) GetOriginalURL(ctx context.Context, shortID string) (string, bool, error) {
	query, args, err := p.sb.
		Select("original_url", "is_deleted").
		From("urls").
		Where(squirrel.Eq{"short_id": shortID}).
		ToSql()
	if err != nil {
		return "", false, fmt.Errorf("build query: %w", err)
	}

	var originalURL string
	var isDeleted bool
	err = p.pool.QueryRow(ctx, query, args...).Scan(&originalURL, &isDeleted)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("query row: %w", err)
	}

	if isDeleted {
		return "", true, nil
	}

	return originalURL, false, nil
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
			return "", nil
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
		Columns("short_id", "original_url", "user_id")
	for _, item := range urls {
		insertBuilder = insertBuilder.Values(item.ShortID, item.OriginalURL, item.UserID)
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

func (p *PostgresRepository) DeleteUserURLs(ctx context.Context, userID string, shortIDs []string) error {
	if len(shortIDs) == 0 {
		return nil
	}

	query, args, err := p.sb.
		Update("urls").
		Set("is_deleted", true).
		Where(squirrel.And{
			squirrel.Eq{"user_id": userID},
			squirrel.Eq{"short_id": shortIDs},
		}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	_, err = p.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete URLs: %w", err)
	}

	return nil
}

func (p *PostgresRepository) GetURLsByShortIDs(ctx context.Context, shortIDs []string) (map[string]models.Storage, error) {
	if len(shortIDs) == 0 {
		return make(map[string]models.Storage), nil
	}

	query, args, err := p.sb.
		Select("short_id", "user_id", "original_url", "is_deleted").
		From("urls").
		Where(squirrel.Eq{"short_id": shortIDs}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query URLs: %w", err)
	}
	defer rows.Close()

	result := make(map[string]models.Storage)
	for rows.Next() {
		var storage models.Storage
		err := rows.Scan(&storage.ShortURL, &storage.UserID, &storage.OriginalURL, &storage.IsDeleted)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		result[storage.ShortURL] = storage
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}

type BatchItem struct {
	ShortID     string
	OriginalURL string
	UserID      string
}
