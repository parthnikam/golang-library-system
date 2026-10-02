package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"library-cli/db"
)

type screen int

const (
	scrHome screen = iota
	scrLogin
	scrRegister
	scrMenu
	scrSearch
	scrLoans
	scrAdd
)

const (
	actLogin = iota
	actRegister
	actQuit
	actSearch
	actLoans
	actAdd
	actLogout
)

const (
	promptCheckout = "checkout"
	promptReturn   = "return"
)

type menuItem struct {
	label string
	act   int
}

type Model struct {
	screen screen
	user   *db.User
	width  int
	height int

	menuIndex int
	focus     int
	addFocus  int
	role      string

	username textinput.Model
	password textinput.Model
	searchIn textinput.Model
	titleIn  textinput.Model
	authorIn textinput.Model
	isbnIn   textinput.Model
	copiesIn textinput.Model

	books       []db.Book
	bookCursor  int
	searchGen   int
	leases      []db.Lease
	leaseCursor int

	prompt     string
	promptKind string
	status     string
	statusErr  bool
	busy       bool
}

type authedMsg struct {
	user *db.User
	err  error
}

type booksMsg struct {
	gen   int
	books []db.Book
	err   error
}

type leasesMsg struct {
	leases []db.Lease
	err    error
}

type checkoutMsg struct {
	lease *db.Lease
	title string
	books []db.Book
	gen   int
	err   error
}

type returnMsg struct {
	title  string
	leases []db.Lease
	err    error
}

type savedMsg struct {
	book *db.Book
	err  error
}

// New is the library screen: register, login, search, and checkout.
func New() Model {
	username := newField("username", 32, 64)
	password := newField("password", 32, 72)
	password.EchoMode = textinput.EchoPassword

	copies := newField("1", 8, 4)
	copies.SetValue("1")

	return Model{
		screen:   scrHome,
		role:     "patron",
		username: username,
		password: password,
		searchIn: newField("title, author, or ISBN", 40, 80),
		titleIn:  newField("title", 40, 120),
		authorIn: newField("author", 40, 120),
		isbnIn:   newField("ISBN", 24, 32),
		copiesIn: copies,
	}
}

func newField(placeholder string, width, limit int) textinput.Model {
	field := textinput.New()
	field.Placeholder = placeholder
	field.Prompt = ""
	field.CharLimit = limit
	field.Width = width
	return field
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.resize(msg), nil
	case authedMsg:
		return m.gotAuth(msg), nil
	case booksMsg:
		return m.gotBooks(msg), nil
	case leasesMsg:
		return m.gotLeases(msg), nil
	case checkoutMsg:
		return m.gotCheckout(msg), nil
	case returnMsg:
		return m.gotReturn(msg), nil
	case savedMsg:
		return m.gotSaved(msg)
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.busy {
			return m, nil
		}
		if m.promptKind != "" {
			return m.answerPrompt(msg)
		}
		return m.onKey(msg)
	}
	return m.pass(msg)
}

func (m Model) resize(msg tea.WindowSizeMsg) Model {
	m.width = msg.Width
	m.height = msg.Height
	width := msg.Width - 4
	if width < 16 {
		width = 16
	}
	if width > 48 {
		width = 48
	}
	m.username.Width = width
	m.password.Width = width
	m.searchIn.Width = width
	m.titleIn.Width = width
	m.authorIn.Width = width
	m.isbnIn.Width = width
	m.copiesIn.Width = 8
	return m
}

func (m Model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case scrHome, scrMenu:
		return m.onMenuKey(msg)
	case scrLogin, scrRegister:
		return m.onAuthKey(msg)
	case scrSearch:
		return m.onSearchKey(msg)
	case scrLoans:
		return m.onLoansKey(msg)
	case scrAdd:
		return m.onAddKey(msg)
	default:
		return m, nil
	}
}

func (m Model) onMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		return m.moveMenu(-1), nil
	case "down", "j":
		return m.moveMenu(1), nil
	case "enter":
		return m.activate()
	case "q":
		return m, tea.Quit
	default:
		return m, nil
	}
}

func (m Model) onAuthKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	last := 1
	if m.screen == scrRegister {
		last = 2
	}
	switch msg.String() {
	case "esc":
		return m.backHome(), nil
	case "tab", "down", "shift+tab", "up":
		delta := 1
		if msg.String() == "shift+tab" || msg.String() == "up" {
			delta = -1
		}
		return m.cycleAuth(delta)
	case "left", "right":
		if m.screen == scrRegister && m.focus == 2 {
			return m.toggleRole(), nil
		}
		return m.pass(msg)
	case "enter":
		if m.focus < last {
			return m.cycleAuth(1)
		}
		if m.screen == scrLogin {
			return m.submitLogin()
		}
		return m.submitRegister()
	default:
		if m.focus == 2 {
			return m, nil
		}
		return m.pass(msg)
	}
}

func (m Model) onSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m.toMenu(), nil
	case "up":
		m.prompt = ""
		return m.moveBook(-1), nil
	case "down":
		m.prompt = ""
		return m.moveBook(1), nil
	case "enter":
		return m.askCheckout()
	default:
		return m.pass(msg)
	}
}

func (m Model) onLoansKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return m.toMenu(), nil
	case "up", "k":
		return m.moveLease(-1), nil
	case "down", "j":
		return m.moveLease(1), nil
	case "enter":
		return m.askReturn()
	default:
		return m, nil
	}
}

func (m Model) onAddKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m.toMenu(), nil
	case "tab", "down", "shift+tab", "up":
		delta := 1
		if msg.String() == "shift+tab" || msg.String() == "up" {
			delta = -1
		}
		return m.cycleAdd(delta)
	case "enter":
		if m.addFocus < 3 {
			return m.cycleAdd(1)
		}
		return m.submitAdd()
	default:
		return m.pass(msg)
	}
}

func (m Model) answerPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		if m.promptKind == promptCheckout {
			return m.doCheckout()
		}
		return m.doReturn()
	case "n", "N", "esc":
		m.prompt = ""
		m.promptKind = ""
		return m, nil
	default:
		return m, nil
	}
}

func (m Model) items() []menuItem {
	if m.screen == scrHome {
		return []menuItem{
			{"Login", actLogin},
			{"Register", actRegister},
			{"Quit", actQuit},
		}
	}
	items := []menuItem{
		{"Search and check out", actSearch},
		{"My checkouts", actLoans},
	}
	if m.user != nil && m.user.Role == "librarian" {
		items = append(items, menuItem{"Add a book", actAdd})
	}
	items = append(items, menuItem{"Log out", actLogout})
	return items
}

func (m Model) moveMenu(delta int) Model {
	items := m.items()
	if len(items) == 0 {
		return m
	}
	m.menuIndex += delta
	if m.menuIndex < 0 {
		m.menuIndex = len(items) - 1
	}
	if m.menuIndex >= len(items) {
		m.menuIndex = 0
	}
	return m
}

func (m Model) activate() (tea.Model, tea.Cmd) {
	items := m.items()
	if m.menuIndex < 0 || m.menuIndex >= len(items) {
		return m, nil
	}
	switch items[m.menuIndex].act {
	case actLogin:
		return m.openLogin()
	case actRegister:
		return m.openRegister()
	case actQuit:
		return m, tea.Quit
	case actSearch:
		return m.openSearch()
	case actLoans:
		return m.openLoans()
	case actAdd:
		return m.openAdd()
	case actLogout:
		return m.logout(), nil
	default:
		return m, nil
	}
}

func (m Model) openLogin() (Model, tea.Cmd) {
	m.screen = scrLogin
	m.focus = 0
	m.clearNotice()
	m.username.Reset()
	m.password.Reset()
	return m.focusAuth()
}

func (m Model) openRegister() (Model, tea.Cmd) {
	m.screen = scrRegister
	m.focus = 0
	m.role = "patron"
	m.clearNotice()
	m.username.Reset()
	m.password.Reset()
	return m.focusAuth()
}

func (m Model) openSearch() (Model, tea.Cmd) {
	m.screen = scrSearch
	m.bookCursor = 0
	m.prompt = ""
	m.promptKind = ""
	m.clearNotice()
	m.searchIn.Reset()
	blink := m.searchIn.Focus()
	m, search := m.searchCmd()
	return m, tea.Batch(blink, search)
}

func (m Model) openLoans() (Model, tea.Cmd) {
	m.screen = scrLoans
	m.leaseCursor = 0
	m.prompt = ""
	m.promptKind = ""
	m.clearNotice()
	m.busy = true
	m.status = "Loading checkouts..."
	userID := m.user.ID
	return m, func() tea.Msg {
		leases, err := db.ListActiveLeases(userID)
		return leasesMsg{leases: leases, err: err}
	}
}

func (m Model) openAdd() (Model, tea.Cmd) {
	m.screen = scrAdd
	m.addFocus = 0
	m.clearNotice()
	m.titleIn.Reset()
	m.authorIn.Reset()
	m.isbnIn.Reset()
	m.copiesIn.SetValue("1")
	return m.focusAdd()
}

func (m Model) logout() Model {
	m.user = nil
	m.screen = scrHome
	m.menuIndex = 0
	m.books = nil
	m.leases = nil
	m.prompt = ""
	m.promptKind = ""
	m.username.Reset()
	m.password.Reset()
	m.clearNotice()
	return m
}

func (m Model) backHome() Model {
	m.screen = scrHome
	m.menuIndex = 0
	m.username.Blur()
	m.password.Blur()
	m.clearNotice()
	return m
}

func (m Model) toMenu() Model {
	m.screen = scrMenu
	m.prompt = ""
	m.promptKind = ""
	m.searchIn.Blur()
	m.titleIn.Blur()
	m.authorIn.Blur()
	m.isbnIn.Blur()
	m.copiesIn.Blur()
	m.clearNotice()
	return m
}

func (m *Model) clearNotice() {
	m.status = ""
	m.statusErr = false
}

func (m Model) cycleAuth(delta int) (Model, tea.Cmd) {
	last := 1
	if m.screen == scrRegister {
		last = 2
	}
	m.focus += delta
	if m.focus > last {
		m.focus = 0
	}
	if m.focus < 0 {
		m.focus = last
	}
	return m.focusAuth()
}

func (m Model) focusAuth() (Model, tea.Cmd) {
	m.username.Blur()
	m.password.Blur()
	switch m.focus {
	case 0:
		return m, m.username.Focus()
	case 1:
		return m, m.password.Focus()
	default:
		return m, nil
	}
}

func (m Model) toggleRole() Model {
	if m.role == "librarian" {
		m.role = "patron"
	} else {
		m.role = "librarian"
	}
	return m
}

func (m Model) cycleAdd(delta int) (Model, tea.Cmd) {
	m.addFocus += delta
	if m.addFocus > 3 {
		m.addFocus = 0
	}
	if m.addFocus < 0 {
		m.addFocus = 3
	}
	return m.focusAdd()
}

func (m Model) focusAdd() (Model, tea.Cmd) {
	m.titleIn.Blur()
	m.authorIn.Blur()
	m.isbnIn.Blur()
	m.copiesIn.Blur()
	switch m.addFocus {
	case 0:
		return m, m.titleIn.Focus()
	case 1:
		return m, m.authorIn.Focus()
	case 2:
		return m, m.isbnIn.Focus()
	default:
		return m, m.copiesIn.Focus()
	}
}

func (m Model) submitLogin() (Model, tea.Cmd) {
	username := strings.TrimSpace(m.username.Value())
	password := m.password.Value()
	if username == "" || password == "" {
		m.status = "Username and password cannot be empty"
		m.statusErr = true
		return m, nil
	}
	m.busy = true
	m.status = "Signing in..."
	m.statusErr = false
	return m, func() tea.Msg {
		user, err := db.AuthenticateUser(username, password)
		return authedMsg{user: user, err: err}
	}
}

func (m Model) submitRegister() (Model, tea.Cmd) {
	username := strings.TrimSpace(m.username.Value())
	password := m.password.Value()
	role := m.role
	if username == "" || password == "" {
		m.status = "Username and password cannot be empty"
		m.statusErr = true
		return m, nil
	}
	m.busy = true
	m.status = "Creating account..."
	m.statusErr = false
	return m, func() tea.Msg {
		user, err := db.RegisterUser(username, password, role)
		return authedMsg{user: user, err: err}
	}
}

func (m Model) submitAdd() (Model, tea.Cmd) {
	title := strings.TrimSpace(m.titleIn.Value())
	author := strings.TrimSpace(m.authorIn.Value())
	isbn := strings.TrimSpace(m.isbnIn.Value())
	copiesText := strings.TrimSpace(m.copiesIn.Value())
	if title == "" || author == "" || isbn == "" {
		m.status = "Title, author, and ISBN are required"
		m.statusErr = true
		return m, nil
	}
	copies, err := strconv.Atoi(copiesText)
	if err != nil || copies < 1 {
		m.status = "Copies must be a number, at least 1"
		m.statusErr = true
		return m, nil
	}
	m.busy = true
	m.status = "Saving book..."
	m.statusErr = false
	return m, func() tea.Msg {
		book, err := db.AddOrUpdateBook(title, author, isbn, copies)
		return savedMsg{book: book, err: err}
	}
}

func (m Model) searchCmd() (Model, tea.Cmd) {
	m.searchGen++
	gen := m.searchGen
	query := m.searchIn.Value()
	return m, func() tea.Msg {
		books, err := db.SearchBooks(query)
		return booksMsg{gen: gen, books: books, err: err}
	}
}

func (m Model) askCheckout() (Model, tea.Cmd) {
	if len(m.books) == 0 || m.bookCursor < 0 || m.bookCursor >= len(m.books) {
		m.status = "No book matches that search"
		m.statusErr = true
		return m, nil
	}
	book := m.books[m.bookCursor]
	if m.user == nil {
		m.status = "Sign in before checking out a book"
		m.statusErr = true
		return m, nil
	}
	if book.AvailableCopies < 1 {
		m.status = "No copies of " + book.Title + " are available"
		m.statusErr = true
		return m, nil
	}
	m.status = ""
	m.statusErr = false
	m.promptKind = promptCheckout
	m.prompt = fmt.Sprintf("Check out %q for user #%d?", book.Title, m.user.ID)
	return m, nil
}

func (m Model) doCheckout() (Model, tea.Cmd) {
	if m.user == nil || m.bookCursor < 0 || m.bookCursor >= len(m.books) {
		m.prompt = ""
		m.promptKind = ""
		m.status = "Sign in before checking out a book"
		m.statusErr = true
		return m, nil
	}
	book := m.books[m.bookCursor]
	m.prompt = ""
	m.promptKind = ""
	m.busy = true
	m.status = "Checking out..."
	m.statusErr = false

	userID := m.user.ID
	bookID := book.ID
	title := book.Title
	gen := m.searchGen
	query := m.searchIn.Value()
	return m, func() tea.Msg {
		lease, err := db.CheckoutBook(userID, bookID)
		if err != nil {
			return checkoutMsg{err: err}
		}
		books, searchErr := db.SearchBooks(query)
		if searchErr != nil {
			return checkoutMsg{lease: lease, title: title, err: searchErr}
		}
		return checkoutMsg{lease: lease, title: title, books: books, gen: gen}
	}
}

func (m Model) askReturn() (Model, tea.Cmd) {
	if len(m.leases) == 0 || m.leaseCursor < 0 || m.leaseCursor >= len(m.leases) {
		m.status = "You have no books checked out"
		m.statusErr = true
		return m, nil
	}
	lease := m.leases[m.leaseCursor]
	m.status = ""
	m.statusErr = false
	m.promptKind = promptReturn
	m.prompt = fmt.Sprintf("Return %q?", lease.Title)
	return m, nil
}

func (m Model) doReturn() (Model, tea.Cmd) {
	if m.user == nil || m.leaseCursor < 0 || m.leaseCursor >= len(m.leases) {
		m.prompt = ""
		m.promptKind = ""
		return m, nil
	}
	lease := m.leases[m.leaseCursor]
	m.prompt = ""
	m.promptKind = ""
	m.busy = true
	m.status = "Returning..."
	m.statusErr = false
	userID := m.user.ID
	leaseID := lease.ID
	title := lease.Title
	return m, func() tea.Msg {
		err := db.ReturnLease(userID, leaseID)
		if err != nil {
			return returnMsg{err: err}
		}
		leases, listErr := db.ListActiveLeases(userID)
		if listErr != nil {
			return returnMsg{title: title, err: listErr}
		}
		return returnMsg{title: title, leases: leases}
	}
}

func (m Model) gotAuth(msg authedMsg) Model {
	m.busy = false
	if msg.err != nil {
		m.status = msg.err.Error()
		m.statusErr = true
		return m
	}
	m.user = msg.user
	m.password.Reset()
	m.username.Reset()
	m.username.Blur()
	m.password.Blur()
	m.screen = scrMenu
	m.menuIndex = 0
	m.status = ""
	m.statusErr = false
	return m
}

func (m Model) gotBooks(msg booksMsg) Model {
	if msg.gen != m.searchGen {
		return m
	}
	if msg.err != nil {
		m.status = msg.err.Error()
		m.statusErr = true
		return m
	}
	var selected int64
	if m.bookCursor >= 0 && m.bookCursor < len(m.books) {
		selected = m.books[m.bookCursor].ID
	}
	m.books = msg.books
	m.bookCursor = 0
	for i, book := range m.books {
		if book.ID == selected {
			m.bookCursor = i
			break
		}
	}
	if m.statusErr {
		m.status = ""
		m.statusErr = false
	}
	return m
}

func (m Model) gotLeases(msg leasesMsg) Model {
	m.busy = false
	if msg.err != nil {
		m.status = msg.err.Error()
		m.statusErr = true
		return m
	}
	m.leases = msg.leases
	if m.leaseCursor >= len(m.leases) {
		m.leaseCursor = 0
	}
	if m.status == "Loading checkouts..." {
		m.status = ""
	}
	return m
}

func (m Model) gotCheckout(msg checkoutMsg) Model {
	m.busy = false
	if msg.lease == nil {
		if msg.err != nil {
			m.status = msg.err.Error()
		}
		m.statusErr = true
		return m
	}
	m.status = fmt.Sprintf("Checked out %s. Lease #%d is logged for user #%d.", msg.title, msg.lease.ID, msg.lease.UserID)
	m.statusErr = false
	if msg.err == nil && msg.gen == m.searchGen {
		m.books = msg.books
		m = m.selectBook(msg.lease.BookID)
	}
	return m
}

func (m Model) gotReturn(msg returnMsg) Model {
	m.busy = false
	if msg.err != nil {
		m.status = msg.err.Error()
		m.statusErr = true
		return m
	}
	m.leases = msg.leases
	if m.leaseCursor >= len(m.leases) {
		m.leaseCursor = 0
	}
	m.status = "Returned " + msg.title + "."
	m.statusErr = false
	return m
}

func (m Model) gotSaved(msg savedMsg) (Model, tea.Cmd) {
	m.busy = false
	if msg.err != nil {
		m.status = msg.err.Error()
		m.statusErr = true
		return m, nil
	}
	m.status = fmt.Sprintf("Saved %s. %d of %d available.", msg.book.Title, msg.book.AvailableCopies, msg.book.TotalCopies)
	m.statusErr = false
	m.titleIn.Reset()
	m.authorIn.Reset()
	m.isbnIn.Reset()
	m.copiesIn.SetValue("1")
	m.addFocus = 0
	return m.focusAdd()
}

func (m Model) selectBook(id int64) Model {
	m.bookCursor = 0
	for i, book := range m.books {
		if book.ID == id {
			m.bookCursor = i
			break
		}
	}
	return m
}

func (m Model) moveBook(delta int) Model {
	if len(m.books) == 0 {
		return m
	}
	m.bookCursor += delta
	if m.bookCursor < 0 {
		m.bookCursor = 0
	}
	if m.bookCursor >= len(m.books) {
		m.bookCursor = len(m.books) - 1
	}
	return m
}

func (m Model) moveLease(delta int) Model {
	if len(m.leases) == 0 {
		return m
	}
	m.leaseCursor += delta
	if m.leaseCursor < 0 {
		m.leaseCursor = 0
	}
	if m.leaseCursor >= len(m.leases) {
		m.leaseCursor = len(m.leases) - 1
	}
	return m
}

func (m Model) pass(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.screen {
	case scrLogin, scrRegister:
		switch m.focus {
		case 0:
			m.username, cmd = m.username.Update(msg)
		case 1:
			m.password, cmd = m.password.Update(msg)
		}
	case scrSearch:
		before := m.searchIn.Value()
		m.searchIn, cmd = m.searchIn.Update(msg)
		if m.searchIn.Value() != before {
			var search tea.Cmd
			m, search = m.searchCmd()
			cmd = tea.Batch(cmd, search)
		}
	case scrAdd:
		switch m.addFocus {
		case 0:
			m.titleIn, cmd = m.titleIn.Update(msg)
		case 1:
			m.authorIn, cmd = m.authorIn.Update(msg)
		case 2:
			m.isbnIn, cmd = m.isbnIn.Update(msg)
		case 3:
			m.copiesIn, cmd = m.copiesIn.Update(msg)
		}
	}
	return m, cmd
}
