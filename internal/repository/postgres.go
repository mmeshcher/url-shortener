package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := createTables(ctx, pool); err != nil {
		return nil, fmt.Errorf("create tables: %w", err)
	}

	return &PostgresRepository{pool: pool}, nil
}

func createTables(ctx context.Context, pool *pgxpool.Pool) error {
	query := `
	CREATE TABLE IF NOT EXISTS urls (
		id VARCHAR(10) PRIMARY KEY,
		original_url TEXT NOT NULL UNIQUE,
		created_at TIMESTAMP DEFAULT NOW()
	);
	
	CREATE INDEX IF NOT EXISTS idx_original_url ON urls(original_url);
	`

	_, err := pool.Exec(ctx, query)
	return err
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
