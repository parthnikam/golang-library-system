package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         string
}

func RegisterUser(username, password, role string) (*User, error) {
	username = strings.TrimSpace(username) // removes spaces
	role = strings.ToLower(strings.TrimSpace(role))

	if username == "" || password == "" {
		return nil, errors.New("Username and Password cannot be null")
	}

	if role != "librarian" && role != "patron" {
		return nil, errors.New("Role must be librarian or patron")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12) // hash password with cost 12
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	query := `INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)`
	result, err := DB.Exec(query, username, string(hash), role)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, errors.New("Username already exists")
		}
		return nil, fmt.Errorf("Could not create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &User{
		ID:       id,
		Username: username,
		Role:     role,
	}, nil
}

func AuthenticateUser(username, password string) (*User, error) {
	username = strings.TrimSpace(username)

	var u User
	query := `SELECT id, username, password_hash, role FROM users WHERE username = ?`
	row := DB.QueryRow(query, username)
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("Invalid username or password")
		}
		return nil, fmt.Errorf("Database query error: %w", err)
	}

	// compare plaintext password against hash
	err = bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	if err != nil {
		return nil, errors.New("Invalid username or password")
	}

	u.PasswordHash = ""
	return &u, nil
}
