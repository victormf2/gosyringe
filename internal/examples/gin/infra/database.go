package infra

import (
	"database/sql"
	_ "embed"
	_ "github.com/mattn/go-sqlite3"
)

type DBConfig struct {
	Path string
}

//go:embed schema.sql
var schema string

func NewDB(config DBConfig) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", config.Path)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(schema)
	if err != nil {
		return nil, err
	}

	return db, err
}
