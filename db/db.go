package db 

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func InitDB(filepath string) (*sql.DB, error) {
	var err error
	DB, err = sql.Open("sqlite", filepath)
	if (err != nil) {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if _, err := DB.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL CHECK(role IN ('librarian', 'patron'))
	);

	CREATE TABLE IF NOT EXISTS books (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		author TEXT NOT NULL,
		isbn TEXT UNIQUE NOT NULL,
		total_copies INTEGER NOT NULL CHECK(total_copies >= 0),
		available_copies INTEGER NOT NULL CHECK(available_copies >= 0)
	);

	CREATE TABLE IF NOT EXISTS leases (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL REFERENCES users(id),
		book_id INTEGER NOT NULL REFERENCES books(id),
		borrowed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		returned_at DATETIME,
		status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'returned'))
	);
	`
	if _, err := DB.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return DB, nil
}
