package db

import (
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "library.db")
	if _, err := InitDB(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if DB != nil {
			DB.Close()
		}
	})
}

func insertUser(t *testing.T, username, role string) int64 {
	t.Helper()
	result, err := DB.Exec(`INSERT INTO users (username, password_hash, role) VALUES (?, 'x', ?)`, username, role)
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestSearchAndCheckout(t *testing.T) {
	openTestDB(t)
	userID := insertUser(t, "bob", "patron")
	otherID := insertUser(t, "ada", "librarian")

	if _, err := AddOrUpdateBook("The Hobbit", "Tolkien", "9780261102217", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := AddOrUpdateBook("100% Done", "Ada", "9780000000001", 2); err != nil {
		t.Fatal(err)
	}

	all, err := SearchBooks("")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("all books: got %d", len(all))
	}

	byAuthor, err := SearchBooks("tolkien")
	if err != nil {
		t.Fatal(err)
	}
	if len(byAuthor) != 1 || byAuthor[0].Title != "The Hobbit" {
		t.Fatalf("author search: %+v", byAuthor)
	}

	byISBN, err := SearchBooks("9780261102217")
	if err != nil {
		t.Fatal(err)
	}
	if len(byISBN) != 1 {
		t.Fatalf("isbn search: %+v", byISBN)
	}

	// A literal percent sign must not match every title.
	percent, err := SearchBooks("%")
	if err != nil {
		t.Fatal(err)
	}
	if len(percent) != 1 || percent[0].Title != "100% Done" {
		t.Fatalf("percent search: %+v", percent)
	}

	none, err := SearchBooks("zzzz-not-a-book")
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("expected no matches, got %+v", none)
	}

	hobbit := byAuthor[0]
	lease, err := CheckoutBook(userID, hobbit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if lease.UserID != userID || lease.BookID != hobbit.ID || lease.Status != "active" {
		t.Fatalf("lease: %+v", lease)
	}

	out, err := ListAllActiveLeases()
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Username != "bob" || out[0].Title != "The Hobbit" || out[0].UserID != userID {
		t.Fatalf("all leases: %+v", out)
	}
	if _, err := CheckoutBook(otherID, percent[0].ID); err != nil {
		t.Fatal(err)
	}
	out, err = ListAllActiveLeases()
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].Username != "ada" || out[0].Title != "100% Done" || out[1].Username != "bob" {
		t.Fatalf("all leases with two borrowers: %+v", out)
	}

	if _, err := CheckoutBook(userID, hobbit.ID); err == nil {
		t.Fatal("expected the last copy to be unavailable")
	}

	if err := ReturnLease(otherID, lease.ID); err == nil {
		t.Fatal("expected another user to be blocked from returning this lease")
	}

	if err := ReturnLease(userID, lease.ID); err != nil {
		t.Fatal(err)
	}
	again, err := CheckoutBook(userID, hobbit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.UserID != userID {
		t.Fatalf("second lease user: %+v", again)
	}

	active, err := ListActiveLeases(userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].UserID != userID || active[0].Username != "bob" {
		t.Fatalf("active leases: %+v", active)
	}
	out, err = ListAllActiveLeases()
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("returned book should leave the other lease: %+v", out)
	}
}
