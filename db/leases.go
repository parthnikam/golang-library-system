package db

import (
	"database/sql"
	"errors"
	"fmt"
)

// Lease is one checkout. UserID is the patron or librarian who took the book.
type Lease struct {
	ID         int64
	UserID     int64
	BookID     int64
	BorrowedAt string
	Status     string
	Title      string
	Author     string
}

// CheckoutBook records a lease for userID and removes one available copy.
func CheckoutBook(userID, bookID int64) (*Lease, error) {
	tx, err := DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("checkout: %w", err)
	}
	defer tx.Rollback()

	var title, author string
	var available int
	err = tx.QueryRow(`SELECT title, author, available_copies FROM books WHERE id = ?`, bookID).
		Scan(&title, &author, &available)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("book not found")
	}
	if err != nil {
		return nil, fmt.Errorf("checkout: %w", err)
	}
	if available < 1 {
		return nil, errors.New("no copies available")
	}

	// The available_copies check makes a second checkout of the last copy fail
	// even if two requests read the old count.
	result, err := tx.Exec(`
		UPDATE books
		SET available_copies = available_copies - 1
		WHERE id = ? AND available_copies > 0`, bookID)
	if err != nil {
		return nil, fmt.Errorf("checkout: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("checkout: %w", err)
	}
	if n == 0 {
		return nil, errors.New("no copies available")
	}

	result, err = tx.Exec(`
		INSERT INTO leases (user_id, book_id, status)
		VALUES (?, ?, 'active')`, userID, bookID)
	if err != nil {
		return nil, fmt.Errorf("checkout: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("checkout: %w", err)
	}

	var borrowedAt string
	err = tx.QueryRow(`SELECT borrowed_at FROM leases WHERE id = ?`, id).Scan(&borrowedAt)
	if err != nil {
		return nil, fmt.Errorf("checkout: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("checkout: %w", err)
	}

	return &Lease{
		ID:         id,
		UserID:     userID,
		BookID:     bookID,
		BorrowedAt: borrowedAt,
		Status:     "active",
		Title:      title,
		Author:     author,
	}, nil
}

// ReturnLease closes an active lease owned by userID and puts the copy back.
func ReturnLease(userID, leaseID int64) error {
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("return: %w", err)
	}
	defer tx.Rollback()

	var bookID int64
	err = tx.QueryRow(`
		SELECT book_id FROM leases
		WHERE id = ? AND user_id = ? AND status = 'active'`, leaseID, userID).Scan(&bookID)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("active checkout not found")
	}
	if err != nil {
		return fmt.Errorf("return: %w", err)
	}

	if _, err := tx.Exec(`
		UPDATE leases
		SET status = 'returned', returned_at = CURRENT_TIMESTAMP
		WHERE id = ?`, leaseID); err != nil {
		return fmt.Errorf("return: %w", err)
	}
	if _, err := tx.Exec(`
		UPDATE books
		SET available_copies = available_copies + 1
		WHERE id = ? AND available_copies < total_copies`, bookID); err != nil {
		return fmt.Errorf("return: %w", err)
	}
	return tx.Commit()
}

// ListActiveLeases returns the books userID currently has checked out.
func ListActiveLeases(userID int64) ([]Lease, error) {
	rows, err := DB.Query(`
		SELECT l.id, l.user_id, l.book_id, l.borrowed_at, l.status, b.title, b.author
		FROM leases l
		JOIN books b ON b.id = l.book_id
		WHERE l.user_id = ? AND l.status = 'active'
		ORDER BY l.id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list checkouts: %w", err)
	}
	defer rows.Close()

	var leases []Lease
	for rows.Next() {
		var lease Lease
		if err := rows.Scan(&lease.ID, &lease.UserID, &lease.BookID, &lease.BorrowedAt, &lease.Status, &lease.Title, &lease.Author); err != nil {
			return nil, err
		}
		leases = append(leases, lease)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return leases, nil
}
