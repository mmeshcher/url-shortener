package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
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

	return &PostgresRepository{pool: pool}, nil
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

func (p *PostgresRepository) SaveURL(ctx context.Context, shortID, originalURL string) error {
	query := `
	INSERT INTO urls (id, original_url) 
	VALUES ($1, $2)
	ON CONFLICT (original_url) DO NOTHING
	`

	_, err := p.pool.Exec(ctx, query, shortID, originalURL)
	return err
}

func (p *PostgresRepository) GetOriginalURL(ctx context.Context, shortID string) (string, error) {
	query := `SELECT original_url FROM urls WHERE id = $1`

	var originalURL string
	err := p.pool.QueryRow(ctx, query, shortID).Scan(&originalURL)
	if err != nil {
		return "", fmt.Errorf("not found: %w", err)
	}

	return originalURL, nil
}

func (p *PostgresRepository) GetShortID(ctx context.Context, originalURL string) (string, error) {
	query := `SELECT id FROM urls WHERE original_url = $1`

	var shortID string
	err := p.pool.QueryRow(ctx, query, originalURL).Scan(&shortID)
	if err != nil {
		return "", fmt.Errorf("not found: %w", err)
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
