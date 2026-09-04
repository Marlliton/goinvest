// Package store é a persistência local: SQLite append-only com proveniência.
// As queries vivem em queries/ e o código de acesso é gerado por sqlc.
package store

import (
	"database/sql"
	"fmt"

	"github.com/marlliton/goinvest/internal/store/gen"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:generate go tool sqlc generate

type DB struct {
	*sql.DB
	q *gen.Queries
}

// Open abre o banco e aplica as migrações pendentes. O usuário nunca roda
// migração à mão.
func Open(path string) (*DB, error) {
	dsn := "file:" + path +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=synchronous(NORMAL)"

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := migrate(sqlDB); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return &DB{DB: sqlDB, q: gen.New(sqlDB)}, nil
}

func migrate(sqlDB *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	// Sem isso o goose escreve "no migrations to run" no log padrão a cada
	// abertura do banco, ou seja, antes da saída de todo comando.
	goose.SetLogger(goose.NopLogger())
	// O driver chama-se "sqlite"; o dialeto do goose continua "sqlite3".
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// String vazia vira NULL: o schema distingue ausência de valor vazio.
func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func intPtr(v *int64) *int {
	if v == nil {
		return nil
	}
	n := int(*v)
	return &n
}
