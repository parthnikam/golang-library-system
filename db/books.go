package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type Book struct {
	ID              int64
	Title           string
	Author          string
	ISBN            string
	TotalCopies     int
	AvailableCopies int
}

// AddOrUpdateBook inserts a new book or increments inventory if the ISBN already exists.
func AddOrUpdateBook(title, author, isbn string, copies int) (*Book, error) {
	title = strings.TrimSpace(title)
	author = strings.TrimSpace(author)
	isbn = strings.TrimSpace(isbn)

	if title == "" || author == "" || isbn == "" {
		return nil, errors.New("title, author, and ISBN cannot be empty")
	}
	if copies <= 0 {
		return nil, errors.New("copies must be at least 1")
	}

	// SQLite upsert: if ISBN exists, add to total and available copies
	query := `
	INSERT INTO books (title, author, isbn, total_copies, available_copies)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(isbn) DO UPDATE SET
		total_copies = total_copies + excluded.total_copies,
		available_copies = available_copies + excluded.available_copies;
	`

	_, err := DB.Exec(query, title, author, isbn, copies, copies)
	if err != nil {
		return nil, fmt.Errorf("failed to save book: %w", err)
	}

	return GetBookByISBN(isbn)
}

// GetBookByID fetches a single book by its primary key.
func GetBookByID(id int64) (*Book, error) {
	query := `SELECT id, title, author, isbn, total_copies, available_copies FROM books WHERE id = ?`
	row := DB.QueryRow(query, id)

	var b Book
	err := row.Scan(&b.ID, &b.Title, &b.Author, &b.ISBN, &b.TotalCopies, &b.AvailableCopies)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("book not found")
		}
		return nil, err
	}
	return &b, nil
}

// GetBookByISBN fetches a single book by its ISBN.
func GetBookByISBN(isbn string) (*Book, error) {
	query := `SELECT id, title, author, isbn, total_copies, available_copies FROM books WHERE isbn = ?`
	row := DB.QueryRow(query, strings.TrimSpace(isbn))

	var b Book
	err := row.Scan(&b.ID, &b.Title, &b.Author, &b.ISBN, &b.TotalCopies, &b.AvailableCopies)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("book not found")
		}
		return nil, err
	}
	return &b, nil
}

// ListBooks returns all books. If availableOnly is true, returns only books with available_copies > 0.
func ListBooks(availableOnly bool) ([]Book, error) {
	query := `SELECT id, title, author, isbn, total_copies, available_copies FROM books`
	if availableOnly {
		query += ` WHERE available_copies > 0`
	}
	query += ` ORDER BY id ASC`
	return queryBooks(query)
}

// SearchBooks matches title, author, or ISBN. An empty query returns every book.
func SearchBooks(query string) ([]Book, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return ListBooks(false)
	}

	pattern := "%" + escapeLike(query) + "%"
	return queryBooks(`
		SELECT id, title, author, isbn, total_copies, available_copies
		FROM books
		WHERE title LIKE ? ESCAPE '\'
		   OR author LIKE ? ESCAPE '\'
		   OR isbn LIKE ? ESCAPE '\'
		ORDER BY title COLLATE NOCASE, id ASC`,
		pattern, pattern, pattern)
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func queryBooks(query string, args ...any) ([]Book, error) {
	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list books: %w", err)
	}
	defer rows.Close()

	var books []Book
	for rows.Next() {
		var b Book
		if err := rows.Scan(&b.ID, &b.Title, &b.Author, &b.ISBN, &b.TotalCopies, &b.AvailableCopies); err != nil {
			return nil, err
		}
		books = append(books, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return books, nil
}
