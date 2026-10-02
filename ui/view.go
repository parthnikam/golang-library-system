package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	"library-cli/db"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("222"))
	labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	selStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("222"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
)

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Library"))
	b.WriteByte('\n')
	if m.user != nil {
		b.WriteString(labelStyle.Render(fmt.Sprintf("%s  ·  %s  ·  user #%d", m.user.Username, m.user.Role, m.user.ID)))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')

	switch m.screen {
	case scrHome:
		b.WriteString("Sign in to search the shelves and check out a book.\n\n")
		b.WriteString(m.viewItems())
	case scrMenu:
		b.WriteString(m.viewItems())
	case scrLogin:
		b.WriteString(m.viewAuth("Login"))
	case scrRegister:
		b.WriteString(m.viewAuth("Create an account"))
	case scrSearch:
		b.WriteString(m.viewSearch())
	case scrLoans:
		b.WriteString(m.viewLoans())
	case scrAllLeases:
		b.WriteString(m.viewAllLeases())
	case scrAdd:
		b.WriteString(m.viewAdd())
	}

	if m.status != "" {
		style := okStyle
		if m.statusErr {
			style = errStyle
		}
		b.WriteByte('\n')
		b.WriteString(style.Render(m.status))
		b.WriteByte('\n')
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render(m.hint()))
	b.WriteByte('\n')
	return b.String()
}

func (m Model) viewItems() string {
	var b strings.Builder
	for i, item := range m.items() {
		line := "  " + item.label
		if i == m.menuIndex {
			b.WriteString(selStyle.Render("> " + item.label))
		} else {
			b.WriteString(line)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func (m Model) viewAuth(heading string) string {
	var b strings.Builder
	b.WriteString(heading)
	b.WriteString("\n\n")
	b.WriteString(fieldLine("Username", m.username, m.focus == 0))
	b.WriteByte('\n')
	b.WriteString(fieldLine("Password", m.password, m.focus == 1))
	if m.screen == scrRegister {
		b.WriteByte('\n')
		b.WriteString(m.viewRole())
	}
	return b.String()
}

func (m Model) viewRole() string {
	patron := "patron"
	librarian := "librarian"
	if m.role == "patron" {
		patron = selStyle.Render(patron)
	} else {
		librarian = selStyle.Render(librarian)
	}
	marker := "  "
	if m.focus == 2 {
		marker = "> "
	}
	return marker + labelStyle.Render("Role") + "\n  " + patron + "    " + librarian + "\n"
}

func (m Model) viewSearch() string {
	var b strings.Builder
	b.WriteString(fieldLine("Search", m.searchIn, true))
	if m.user != nil {
		b.WriteString(dimStyle.Render(fmt.Sprintf("A checkout stores user #%d on the lease.", m.user.ID)))
		b.WriteString("\n\n")
	}
	if len(m.books) == 0 {
		if strings.TrimSpace(m.searchIn.Value()) == "" {
			b.WriteString("No books yet. A librarian can add some.\n")
		} else {
			b.WriteString("No books match that search.\n")
		}
		return b.String()
	}

	height := m.visibleCount()
	start := 0
	if m.bookCursor >= height {
		start = m.bookCursor - height + 1
	}
	end := start + height
	if end > len(m.books) {
		end = len(m.books)
	}
	for i := start; i < end; i++ {
		b.WriteString(bookRow(m.books[i], i == m.bookCursor, m.width))
	}
	return b.String()
}

func bookRow(book db.Book, selected bool, width int) string {
	titleWidth := width - 8
	if titleWidth < 12 {
		titleWidth = 40
	}
	marker := "  "
	if selected {
		marker = "> "
	}
	title := fmt.Sprintf("%s%d  %s", marker, book.ID, clip(book.Title, titleWidth))
	detail := fmt.Sprintf("    %s  ·  %s  ·  %d of %d available", book.Author, book.ISBN, book.AvailableCopies, book.TotalCopies)
	if selected {
		title = selStyle.Render(title)
		detail = selStyle.Render(detail)
	} else if book.AvailableCopies < 1 {
		title = dimStyle.Render(title)
		detail = dimStyle.Render(detail)
	}
	return title + "\n" + detail + "\n"
}

func (m Model) viewLoans() string {
	return m.viewLeaseList("Books you have checked out", "You have no books checked out.", false)
}

func (m Model) viewAllLeases() string {
	return m.viewLeaseList("Books currently leased", "No books are leased right now.", true)
}

func (m Model) viewLeaseList(heading, empty string, showBorrower bool) string {
	var b strings.Builder
	b.WriteString(heading)
	b.WriteString("\n\n")
	if len(m.leases) == 0 && !m.busy {
		b.WriteString(empty)
		b.WriteByte('\n')
		return b.String()
	}
	height := m.visibleCount()
	start := 0
	if m.leaseCursor >= height {
		start = m.leaseCursor - height + 1
	}
	end := start + height
	if end > len(m.leases) {
		end = len(m.leases)
	}
	for i := start; i < end; i++ {
		lease := m.leases[i]
		marker := "  "
		title := fmt.Sprintf("%s#%d  %s", marker, lease.ID, lease.Title)
		detail := fmt.Sprintf("    %s  ·  user #%d  ·  %s", lease.Author, lease.UserID, lease.BorrowedAt)
		if showBorrower {
			detail = fmt.Sprintf("    %s  ·  user #%d  ·  %s  ·  %s", lease.Username, lease.UserID, lease.Author, lease.BorrowedAt)
		}
		if i == m.leaseCursor {
			marker = "> "
			title = selStyle.Render(fmt.Sprintf("%s#%d  %s", marker, lease.ID, lease.Title))
			detail = selStyle.Render(detail)
		}
		b.WriteString(title)
		b.WriteByte('\n')
		b.WriteString(detail)
		b.WriteByte('\n')
	}
	return b.String()
}

func (m Model) viewAdd() string {
	var b strings.Builder
	b.WriteString("Add a book. The same ISBN adds more copies.\n\n")
	b.WriteString(fieldLine("Title", m.titleIn, m.addFocus == 0))
	b.WriteByte('\n')
	b.WriteString(fieldLine("Author", m.authorIn, m.addFocus == 1))
	b.WriteByte('\n')
	b.WriteString(fieldLine("ISBN", m.isbnIn, m.addFocus == 2))
	b.WriteByte('\n')
	b.WriteString(fieldLine("Copies", m.copiesIn, m.addFocus == 3))
	return b.String()
}

func fieldLine(label string, input textinput.Model, focused bool) string {
	marker := "  "
	if focused {
		marker = "> "
	}
	return marker + labelStyle.Render(label) + "\n  " + input.View() + "\n"
}

func (m Model) hint() string {
	if m.busy {
		return "please wait"
	}
	if m.promptKind != "" {
		return m.prompt + "    y yes    n no    esc cancel"
	}
	switch m.screen {
	case scrHome:
		return "↑↓ move    enter select    q quit"
	case scrMenu:
		return "↑↓ move    enter select    q quit"
	case scrLogin:
		return "tab next field    enter submit    esc back"
	case scrRegister:
		return "tab next field    ←→ change role    enter submit    esc back"
	case scrSearch:
		return "type to search    ↑↓ choose    enter check out    esc back"
	case scrLoans:
		return "↑↓ choose    enter return    esc back"
	case scrAllLeases:
		return "↑↓ move    esc back"
	case scrAdd:
		return "tab next field    enter on copies saves    esc back"
	default:
		return "ctrl+c quits"
	}
}

func (m Model) visibleCount() int {
	room := m.height - 12
	if room < 4 {
		return 4
	}
	count := room / 2
	if count > 8 {
		return 8
	}
	return count
}

func clip(s string, n int) string {
	runes := []rune(s)
	if n <= 0 || len(runes) <= n {
		return s
	}
	if n < 2 {
		return string(runes[:n])
	}
	return string(runes[:n-1]) + "…"
}
