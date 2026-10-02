package ui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/crypto/bcrypt"

	"library-cli/db"
)

func TestLoginSearchCheckout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.db")
	if _, err := db.InitDB(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.DB.Close() })

	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec(
		`INSERT INTO users (username, password_hash, role) VALUES (?, ?, 'patron')`,
		"bob", string(hash),
	); err != nil {
		t.Fatal(err)
	}
	book, err := db.AddOrUpdateBook("The Hobbit", "Tolkien", "9780261102217", 1)
	if err != nil {
		t.Fatal(err)
	}

	m := New()
	m.screen = scrLogin
	m.focus = 1
	m.username.SetValue("bob")
	m.password.SetValue("wrong")
	m, cmd := update(t, m, key("enter"))
	m = run(t, m, cmd)
	if m.user != nil || !m.statusErr {
		t.Fatalf("wrong password should fail, user=%v status=%q", m.user, m.status)
	}

	m.password.SetValue("secret")
	m, cmd = update(t, m, key("enter"))
	m = run(t, m, cmd)
	if m.user == nil || m.user.Username != "bob" || m.screen != scrMenu {
		t.Fatalf("login: user=%v screen=%v status=%q", m.user, m.screen, m.status)
	}

	m, cmd = update(t, m, key("enter"))
	m = run(t, m, cmd)
	if m.screen != scrSearch || len(m.books) != 1 {
		t.Fatalf("search: screen=%v books=%d status=%q", m.screen, len(m.books), m.status)
	}

	m, cmd = update(t, m, key("enter"))
	if cmd != nil || m.promptKind != promptCheckout {
		t.Fatalf("expected checkout confirmation, prompt=%q cmd=%v", m.prompt, cmd != nil)
	}
	m, cmd = update(t, m, key("y"))
	m = run(t, m, cmd)
	if m.statusErr {
		t.Fatal(m.status)
	}

	leases, err := db.ListActiveLeases(m.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(leases) != 1 || leases[0].UserID != m.user.ID || leases[0].BookID != book.ID {
		t.Fatalf("lease was not logged for the signed-in user: %+v", leases)
	}

	m, cmd = m.openLoans()
	m = run(t, m, cmd)
	m, cmd = update(t, m, key("enter"))
	if cmd != nil || m.promptKind != promptReturn {
		t.Fatalf("expected return confirmation, prompt=%q", m.prompt)
	}
	m, cmd = update(t, m, key("y"))
	m = run(t, m, cmd)
	if m.statusErr {
		t.Fatal(m.status)
	}
	left, err := db.ListActiveLeases(m.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Fatalf("expected the book to be returned, still have %+v", left)
	}
}

func update(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

func run(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command")
	}
	return dispatch(t, m, cmd())
}

// dispatch runs a command result once. A batch is unwrapped so search can
// load books without following the cursor-blink command forever.
func dispatch(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, cmd := range batch {
			if cmd != nil {
				m = dispatch(t, m, cmd())
			}
		}
		return m
	}
	next, _ := m.Update(msg)
	return next.(Model)
}

func key(name string) tea.KeyMsg {
	if name == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
}
