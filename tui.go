package main

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#e94560")).
			Background(lipgloss.Color("#16213e")).
			Padding(0, 1)

	roomStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0f3460")).
			Background(lipgloss.Color("#16213e"))

	onlineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#53d769")).
			Background(lipgloss.Color("#16213e"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666680")).
			Background(lipgloss.Color("#16213e"))

	inputBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#e94560")).
			Padding(0, 1)

	editBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#ffd700")).
			Padding(0, 1)

	replyBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4d94ff")).
			Padding(0, 1)

	usernameStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#e94560"))

	myUsernameStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#53d769"))

	timeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#444466"))

	msgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c9c9e0"))

	replyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666680")).
			Italic(true)

	replyNameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888899")).
			Bold(true)

	imgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4d94ff")).
			Italic(true)

	mentionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffd700")).
			Bold(true)

	deletedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#444466")).
			Italic(true).
			Strikethrough(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff4757")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#444466"))

	modeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffd700")).
			Bold(true)

	selectMarker = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4d94ff")).
			Bold(true)

	myMsgMarkerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#2d7a2d"))
)

var (
	// Matches an explicit "color: #hex" CSS property.
	renderedColorRe = regexp.MustCompile(`color:\s*#([0-9a-fA-F]{3,8})`)
	// Matches an explicit "color: rgb(r,g,b)" CSS property.
	renderedRGBRe = regexp.MustCompile(`color:\s*rgb\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\)`)
	// Matches an explicit "color: rgba(r,g,b,a)" CSS property.
	renderedRGBARe = regexp.MustCompile(`color:\s*rgba\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*[\d.]+\s*\)`)
	// Matches any #RRGGBB hex color anywhere in the HTML.
	anyHexColorRe = regexp.MustCompile(`#([0-9a-fA-F]{6})`)
	// Matches any rgba(...) occurrence anywhere in the HTML (for gradient backgrounds).
	anyRGBAColorRe = regexp.MustCompile(`rgba\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*[\d.]+\s*\)`)
	rainbowColors  = []lipgloss.Color{
		"196", "214", "226", "46", "51", "93",
	}
)

func isForumStaff(user ChatUser) bool {
	return user.IsAdmin || user.IsSuperAdmin || user.IsMod || user.IsStaff
}

func isUniq(user ChatUser) bool {
	if user.UserTitle == "Уник" {
		return true
	}
	if user.DisplayIconGroupID == 265 {
		return true
	}
	if user.UniqUsernameCss != "" {
		return true
	}
	if strings.Contains(user.Rendered.Username, "uniqUsernameIcon--custom") {
		return true
	}
	if strings.Contains(user.Rendered.Username, "text-shadow") {
		return true
	}
	return false
}

// isUsableColor returns true when r,g,b represents a color that is
// neither too-white (invisible on light bg) nor too-black.
func isUsableColor(r, g, b int) bool {
	if r < 15 && g < 15 && b < 15 {
		return false
	}
	lum := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
	if lum > 210 {
		return false
	}
	return true
}

func rgbToHex(r, g, b int) string {
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func extractRenderedColor(html string) string {
	// 1. Explicit CSS color property: #hex
	if m := renderedColorRe.FindStringSubmatch(html); len(m) == 2 {
		hex := strings.ToLower(m[1])
		if hex != "ffffff" && hex != "000000" {
			return "#" + m[1]
		}
	}
	// 2. Explicit CSS color property: rgb(r,g,b)
	if m := renderedRGBRe.FindStringSubmatch(html); len(m) == 4 {
		r, _ := strconv.Atoi(m[1])
		g, _ := strconv.Atoi(m[2])
		b, _ := strconv.Atoi(m[3])
		if isUsableColor(r, g, b) {
			return rgbToHex(r, g, b)
		}
	}
	// 3. Explicit CSS color property: rgba(r,g,b,a)
	if m := renderedRGBARe.FindStringSubmatch(html); len(m) >= 4 {
		r, _ := strconv.Atoi(m[1])
		g, _ := strconv.Atoi(m[2])
		b, _ := strconv.Atoi(m[3])
		if isUsableColor(r, g, b) {
			return rgbToHex(r, g, b)
		}
	}
	return ""
}

// extractGradientColor tries to find a usable color from gradient backgrounds
// or any hex color in the HTML. Used for non-Uniq users with custom nicks.
func extractGradientColor(html string) string {
	for _, m := range anyRGBAColorRe.FindAllStringSubmatch(html, -1) {
		r, _ := strconv.Atoi(m[1])
		g, _ := strconv.Atoi(m[2])
		b, _ := strconv.Atoi(m[3])
		if isUsableColor(r, g, b) {
			return rgbToHex(r, g, b)
		}
	}
	for _, m := range anyHexColorRe.FindAllStringSubmatch(html, -1) {
		c := strings.ToLower(m[1])
		if c == "ffffff" || c == "000000" {
			continue
		}
		return "#" + m[1]
	}
	return ""
}

func renderUsername(user ChatUser, isMe bool, simpleMode bool) string {
	name := user.Username
	bold := lipgloss.NewStyle().Bold(true)

	if isMe {
		return myUsernameStyle.Render(name)
	}
	if simpleMode {
		return usernameStyle.Render(name)
	}
	if isForumStaff(user) {
		if c := extractRenderedColor(user.Rendered.Username); c != "" {
			return bold.Foreground(lipgloss.Color(c)).Render(name)
		}
		return usernameStyle.Render(name)
	}
	if isUniq(user) {
		var rb strings.Builder
		for i, r := range []rune(name) {
			c := rainbowColors[i%len(rainbowColors)]
			rb.WriteString(bold.Foreground(c).Render(string(r)))
		}
		return rb.String()
	}
	if user.Rendered.Username != "" {
		if c := extractRenderedColor(user.Rendered.Username); c != "" {
			return bold.Foreground(lipgloss.Color(c)).Render(name)
		}
		if c := extractGradientColor(user.Rendered.Username); c != "" {
			return bold.Foreground(lipgloss.Color(c)).Render(name)
		}
	}
	return usernameStyle.Render(name)
}

type tickMsg time.Time
type messagesMsg []ChatMessage
type errorMsg struct{ err error }
type sentMsg struct{}
type editedMsg struct{}
type imageReadyMsg struct{ url string }

func (e errorMsg) Error() string { return e.err.Error() }

type inputMode int

const (
	modeNormal inputMode = iota
	modeEdit
	modeReply
	modeSelect
)

type model struct {
	cfg        Config
	api        *APIClient
	myUserID   int
	myUsername string
	roomID     int
	roomTitle  string

	messages   []ChatMessage
	msgIndex   map[int]int
	viewport   viewport.Model
	input      textinput.Model
	width      int
	height     int
	err        string
	sending    bool
	connected  bool
	lastPoll   time.Time
	msgCount   int
	autoScroll bool

	mode         inputMode
	editMsgID    int
	replyMsgID   int
	replyPreview string
	selectIdx    int

	imgRenderer  *ImageRenderer
	ueberzug     *UeberzugManager
	imgPositions []imgViewPos
}

func initialModel(cfg Config, api *APIClient, myUserID int, myUsername string, roomID int, roomTitle string) model {
	ti := textinput.New()
	ti.Placeholder = "Напиши сообщение..."
	ti.Focus()
	ti.CharLimit = 1000
	ti.Width = 60

	vp := viewport.New(80, 20)

	ir, uz := initRenderer(cfg.ImageMode, cfg.ImageHeight)

	return model{
		cfg:        cfg,
		api:        api,
		myUserID:   myUserID,
		myUsername: myUsername,
		roomID:     roomID,
		roomTitle:  roomTitle,
		input:      ti,
		viewport:   vp,
		msgIndex:   make(map[int]int),
		autoScroll: true,
		mode:       modeNormal,
		selectIdx:  -1,

		imgRenderer:  ir,
		ueberzug:     uz,
		imgPositions: make([]imgViewPos, 0, 8),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.fetchInitialMessages(),
		m.pollMessages(),
	)
}

func (m model) pollMessages() tea.Cmd {
	return tea.Tick(time.Duration(m.cfg.PollMs)*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) fetchMessages() tea.Cmd {
	return func() tea.Msg {
		msgs, err := m.api.GetMessages(m.roomID)
		if err != nil {
			return errorMsg{err}
		}
		return messagesMsg(msgs)
	}
}

func (m model) fetchInitialMessages() tea.Cmd {
	return func() tea.Msg {
		msgs, err := m.api.GetMessages(m.roomID)
		if err != nil {
			return errorMsg{err}
		}
		minID := 0
		for _, msg := range msgs {
			if minID == 0 || msg.MessageID < minID {
				minID = msg.MessageID
			}
		}
		if minID > 0 {
			older, err := m.api.GetMessagesBefore(m.roomID, minID)
			if err == nil {
				msgs = append(older, msgs...)
			}
		}
		return messagesMsg(msgs)
	}
}

func (m model) sendMessage(text string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.api.SendMessage(m.roomID, text)
		if err != nil {
			return errorMsg{err}
		}
		return sentMsg{}
	}
}

func (m model) replyMessage(replyID int, text string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.api.ReplyMessage(m.roomID, replyID, text)
		if err != nil {
			return errorMsg{err}
		}
		return sentMsg{}
	}
}

func (m model) editMessage(msgID int, text string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.api.EditMessage(msgID, text)
		if err != nil {
			return errorMsg{err}
		}
		return editedMsg{}
	}
}

func (m *model) recalcViewport() {
	if m.width == 0 || m.height == 0 {
		return
	}
	extra := 0
	if m.mode != modeNormal {
		extra = 1
	}
	vpW := m.width - 2
	if vpW < 20 {
		vpW = 20
	}
	vpH := m.height - 1 - 3 - 1 - extra - 2
	if vpH < 3 {
		vpH = 3
	}
	m.viewport.Width = vpW
	m.viewport.Height = vpH
	m.input.Width = vpW - 4
}

// isPunct returns true for punctuation/symbols that should not start a new line.
func isPunct(r rune) bool {
	switch r {
	case '.', ',', '!', '?', ':', ';', ')', ']', '}', '»', '…', '-', '—', '\'', '"', ' ':
		return true
	}
	return false
}

// wrapText breaks s into lines of at most w visible runes,
// pulling trailing punctuation from the next line onto the current one.
func wrapText(s string, w int) []string {
	if w <= 0 {
		return []string{s}
	}
	runes := []rune(s)
	if len(runes) <= w {
		return []string{s}
	}
	var lines []string
	for len(runes) > w {
		cut := w
		// pull punctuation that would start the next line onto this one
		for cut < len(runes) && isPunct(runes[cut]) {
			cut++
		}
		lines = append(lines, string(runes[:cut]))
		runes = runes[cut:]
	}
	if len(runes) > 0 {
		lines = append(lines, string(runes))
	}
	return lines
}

func (m model) msgLineCount(msg ChatMessage) int {
	if msg.IsDeleted {
		return 1
	}
	msgW := m.viewport.Width - 2 // usable width inside viewport
	n := 0
	if msg.Reply != nil {
		n++
	}
	if isImageMessage(msg.MessageRaw) {
		if m.imgRenderer != nil && m.imgRenderer.backend != ImgBackendNone {
			imgH := m.imgRenderer.imgH + 2 // +2 for border top/bottom
			return n + 1 + imgH // header line + bordered art lines
		}
		return n + 1 // just the marker line
	}
	text := cleanMessage(msg.MessageRaw, msg.Message)
	indent := 2 + 5 + 1 + len([]rune(msg.User.Username)) + 2
	avail := msgW - indent
	if avail < 10 {
		avail = 10
	}
	for _, line := range strings.Split(text, "\n") {
		w := len([]rune(line))
		if w == 0 {
			n++
		} else {
			n += (w + avail - 1) / avail
		}
	}
	return n
}

func (m *model) scrollToSelected() {
	if m.selectIdx < 0 || m.selectIdx >= len(m.messages) {
		return
	}
	line := 0
	for i := 0; i < m.selectIdx; i++ {
		line += m.msgLineCount(m.messages[i])
	}
	vpH := m.viewport.Height
	if line < m.viewport.YOffset {
		m.viewport.YOffset = line
	} else if line >= m.viewport.YOffset+vpH {
		m.viewport.YOffset = line - vpH + 1
	}
	m.updateImageOverlays()
}

func (m *model) findLastMyMessage() (int, string) {
	for i := len(m.messages) - 1; i >= 0; i-- {
		msg := m.messages[i]
		if msg.User.UserID == m.myUserID && !msg.IsDeleted {
			text := cleanMessage(msg.MessageRaw, msg.Message)
			return msg.MessageID, text
		}
	}
	return 0, ""
}

func (m *model) msgPreview(idx int) string {
	if idx < 0 || idx >= len(m.messages) {
		return ""
	}
	msg := m.messages[idx]
	text := cleanMessage(msg.MessageRaw, msg.Message)
	preview := msg.User.Username + ": " + text
	if len([]rune(preview)) > 60 {
		preview = string([]rune(preview)[:60]) + "\u2026"
	}
	return preview
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	passToInput := true

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.doCleanup()
			return m, tea.Quit
		case "esc":
			if m.mode != modeNormal {
				m.mode = modeNormal
				m.editMsgID = 0
				m.replyMsgID = 0
				m.replyPreview = ""
				m.selectIdx = -1
				m.input.SetValue("")
				m.input.Placeholder = "Напиши сообщение..."
				m.input.Focus()
				m.recalcViewport()
				m.viewport.SetContent(m.renderMessages())
				m.updateImageOverlays()
			} else {
				m.doCleanup()
				return m, tea.Quit
			}
		case "enter":
			if m.mode == modeSelect {
				if m.selectIdx >= 0 && m.selectIdx < len(m.messages) {
					selected := m.messages[m.selectIdx]
					m.mode = modeReply
					m.replyMsgID = selected.MessageID
					m.replyPreview = m.msgPreview(m.selectIdx)
					m.selectIdx = -1
					m.input.Placeholder = "Ответ..."
					m.input.SetValue("")
					m.input.Focus()
					m.recalcViewport()
					m.viewport.SetContent(m.renderMessages())
					m.updateImageOverlays()
				}
			} else {
				text := strings.TrimSpace(m.input.Value())
				if text != "" && !m.sending {
					m.sending = true
					m.input.SetValue("")
					switch m.mode {
					case modeEdit:
						cmds = append(cmds, m.editMessage(m.editMsgID, text))
						m.mode = modeNormal
						m.editMsgID = 0
						m.input.Placeholder = "Напиши сообщение..."
						m.recalcViewport()
					case modeReply:
						cmds = append(cmds, m.replyMessage(m.replyMsgID, text))
						m.mode = modeNormal
						m.replyMsgID = 0
						m.replyPreview = ""
						m.input.Placeholder = "Напиши сообщение..."
						m.recalcViewport()
					default:
						cmds = append(cmds, m.sendMessage(text))
					}
				}
			}
		case "tab":
			if m.mode == modeNormal && len(m.messages) > 0 {
				m.mode = modeSelect
				m.selectIdx = len(m.messages) - 1
				m.autoScroll = false
				m.input.Blur()
				m.recalcViewport()
				m.viewport.SetContent(m.renderMessages())
				m.viewport.GotoBottom()
				m.updateImageOverlays()
			}
		case "up":
			passToInput = false
			if m.mode == modeSelect {
				if m.selectIdx > 0 {
					m.selectIdx--
					m.viewport.SetContent(m.renderMessages())
					m.scrollToSelected()
				}
			} else {
				m.autoScroll = false
				m.viewport.LineUp(1)
				m.updateImageOverlays()
			}
		case "down":
			passToInput = false
			if m.mode == modeSelect {
				if m.selectIdx < len(m.messages)-1 {
					m.selectIdx++
					m.viewport.SetContent(m.renderMessages())
					m.scrollToSelected()
				}
			} else {
				m.viewport.LineDown(1)
				if m.viewport.AtBottom() {
					m.autoScroll = true
				}
				m.updateImageOverlays()
			}
		case "pgup":
			passToInput = false
			m.autoScroll = false
			m.viewport.HalfViewUp()
			m.updateImageOverlays()
		case "pgdown":
			passToInput = false
			m.viewport.HalfViewDown()
			if m.viewport.AtBottom() {
				m.autoScroll = true
			}
			m.updateImageOverlays()
		case "ctrl+e":
			if m.mode == modeNormal {
				msgID, text := m.findLastMyMessage()
				if msgID > 0 {
					m.mode = modeEdit
					m.editMsgID = msgID
					m.input.SetValue(text)
					m.input.Placeholder = "Редактирование..."
					m.input.CursorEnd()
					m.recalcViewport()
				}
			}
		case "ctrl+r":
			if m.mode == modeNormal && len(m.messages) > 0 {
				last := len(m.messages) - 1
				m.mode = modeReply
				m.replyMsgID = m.messages[last].MessageID
				m.replyPreview = m.msgPreview(last)
				m.input.Placeholder = "Ответ..."
				m.input.SetValue("")
				m.recalcViewport()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcViewport()
		if m.imgRenderer != nil {
			m.imgRenderer.InvalidateRenderCache()
			cmds = append(cmds, m.queuePendingImages()...)
		}
		m.viewport.SetContent(m.renderMessages())
		m.updateImageOverlays()

	case tickMsg:
		cmds = append(cmds, m.fetchMessages())
		cmds = append(cmds, m.pollMessages())

	case messagesMsg:
		m.connected = true
		m.lastPoll = time.Now()
		m.err = ""
		newMsgs := []ChatMessage(msg)
		changed := m.mergeMessages(newMsgs)
		if changed {
			cmds = append(cmds, m.queuePendingImages()...)
			if m.mode == modeSelect {
				savedOffset := m.viewport.YOffset
				m.viewport.SetContent(m.renderMessages())
				m.viewport.YOffset = savedOffset
				m.scrollToSelected()
			} else {
				m.viewport.SetContent(m.renderMessages())
				if m.autoScroll {
					m.viewport.GotoBottom()
				}
			}
			m.updateImageOverlays()
		}

	case imageReadyMsg:
		m.viewport.SetContent(m.renderMessages())
		if m.autoScroll {
			m.viewport.GotoBottom()
		}
		m.updateImageOverlays()

	case sentMsg:
		m.sending = false
		cmds = append(cmds, m.fetchMessages())

	case editedMsg:
		m.sending = false
		cmds = append(cmds, m.fetchMessages())

	case errorMsg:
		m.err = msg.Error()
		m.sending = false
	}

	if passToInput {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) mergeMessages(incoming []ChatMessage) bool {
	changed := false
	for _, msg := range incoming {
		if idx, exists := m.msgIndex[msg.MessageID]; exists {
			old := m.messages[idx]
			if old.Message != msg.Message || old.IsDeleted != msg.IsDeleted || old.MessageRaw != msg.MessageRaw {
				m.messages[idx] = msg
				changed = true
			}
		} else {
			m.msgIndex[msg.MessageID] = len(m.messages)
			m.messages = append(m.messages, msg)
			changed = true
		}
	}

	if changed {
		sort.Slice(m.messages, func(i, j int) bool {
			return m.messages[i].MessageID < m.messages[j].MessageID
		})

		for i, msg := range m.messages {
			m.msgIndex[msg.MessageID] = i
		}

		if len(m.messages) > m.cfg.MaxHistory {
			excess := len(m.messages) - m.cfg.MaxHistory
			for _, msg := range m.messages[:excess] {
				delete(m.msgIndex, msg.MessageID)
			}
			m.messages = m.messages[excess:]
			for i, msg := range m.messages {
				m.msgIndex[msg.MessageID] = i
			}
		}
		m.msgCount = len(m.messages)
	}
	return changed
}

func (m *model) renderMessages() string {
	// Reset image position tracking
	m.imgPositions = m.imgPositions[:0]

	if len(m.messages) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#444466")).
			Render("  Загрузка сообщений...")
	}

	var sb strings.Builder
	currentLine := 0

	for idx, msg := range m.messages {
		selected := m.mode == modeSelect && idx == m.selectIdx
		isOwn := msg.User.UserID == m.myUserID
		prefix := "  "
		if selected {
			prefix = selectMarker.Render("> ")
		} else if isOwn {
			prefix = myMsgMarkerStyle.Render("▍") + " "
		}

		if msg.IsDeleted {
			sb.WriteString(prefix + deletedStyle.Render("[удалено]"))
			sb.WriteString("\n")
			currentLine++
			continue
		}

		t := time.Unix(msg.Date, 0)
		timeStr := timeStyle.Render(t.Format("15:04"))
		nameStr := renderUsername(msg.User, isOwn, m.cfg.SimpleMode)

		if msg.Reply != nil {
			replyText := cleanMessage(msg.Reply.MessageRaw, msg.Reply.Message)
			if replyText == "" || replyText == "[изображение]" {
				replyText = "📷 изображение"
			}
			replyPrefix := "  " + replyStyle.Render("╭ ") + replyNameStyle.Render(msg.Reply.User.Username) + replyStyle.Render(": ")
			replyIndent := 2 + 2 + len([]rune(msg.Reply.User.Username)) + 2 // "  ╭ " + name + ": "
			replyAvail := m.viewport.Width - replyIndent
			if replyAvail < 10 {
				replyAvail = 10
			}
			replyWrapped := wrapText(replyText, replyAvail)
			replyIndentStr := strings.Repeat(" ", replyIndent)
			for ri, rl := range replyWrapped {
				if ri == 0 {
					sb.WriteString(replyPrefix + replyStyle.Render(rl) + "\n")
				} else {
					sb.WriteString(replyIndentStr + replyStyle.Render(rl) + "\n")
				}
			}
		}

		// prefix(2) + time "HH:MM"(5) + space(1) + username + "  "(2)
		indent := 2 + 5 + 1 + len([]rune(msg.User.Username)) + 2

		if isImageMessage(msg.MessageRaw) {
			url := extractImageURL(msg.MessageRaw)
			if url != "" {
				artWidth := m.viewport.Width - indent
				if artWidth < 10 {
					artWidth = 10
				}
				indentStr := strings.Repeat(" ", indent)
				useRenderer := m.imgRenderer != nil && m.imgRenderer.backend != ImgBackendNone

				if useRenderer {
					// Record where the art block starts in the full viewport content
					replyOff := 0
					if msg.Reply != nil {
						replyOff = 1
					}
					m.imgPositions = append(m.imgPositions, imgViewPos{
						msgID:    msg.MessageID,
						url:      url,
						vpLine:   currentLine + replyOff + 1, // +1 for header line
						artWidth: artWidth,
						indent:   indent,
					})

					// Header line: time + username + compact marker (no URL)
					sb.WriteString(fmt.Sprintf("%s%s %s  %s\n",
						prefix, timeStr, nameStr, imgStyle.Render("📷")))

					// Get or build bordered image lines
					var borderedLines []string
					if m.imgRenderer.backend == ImgBackendChafa || m.imgRenderer.backend == ImgBackendKitty {
						artLines := m.imgRenderer.GetRendered(url, artWidth)
						if artLines != nil {
							borderedLines = m.imgRenderer.BorderWrap(artLines, artWidth)
						}
					}
					if borderedLines == nil {
						if m.imgRenderer.backend == ImgBackendUeberzug {
							// Empty bordered space — ueberzug renders pixels on top
							emptyLine := strings.Repeat(" ", artWidth-2)
							artLines := make([]string, m.imgRenderer.imgH)
							for i := range artLines {
								artLines[i] = emptyLine
							}
							borderedLines = m.imgRenderer.BorderWrap(artLines, artWidth)
						} else {
							borderedLines = m.imgRenderer.Placeholder(url, artWidth)
						}
					}
					for _, l := range borderedLines {
						sb.WriteString(indentStr + l + "\n")
					}
				} else {
					// No renderer — clickable hyperlink marker (Ctrl+Click to open)
					hyperlink := fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\",
						url, imgStyle.Render("📷 изображение"))
					sb.WriteString(fmt.Sprintf("%s%s %s  %s\n",
						prefix, timeStr, nameStr, hyperlink))
				}
			}
			currentLine += m.msgLineCount(msg)
			continue
		}

		text := cleanMessage(msg.MessageRaw, msg.Message)

		isMention := strings.Contains(msg.MessageRaw, m.myUsername) ||
			strings.Contains(msg.Message, fmt.Sprintf("@%s", m.myUsername))

		indentStr := strings.Repeat(" ", indent)
		availW := m.viewport.Width - indent
		if availW < 10 {
			availW = 10
		}
		lines := strings.Split(text, "\n")
		firstLine := true
		for _, line := range lines {
			wrapped := wrapText(line, availW)
			if len(wrapped) == 0 {
				wrapped = []string{""}
			}
			for _, wl := range wrapped {
				var content string
				if isMention && msg.User.UserID != m.myUserID {
					content = mentionStyle.Render(wl)
				} else {
					content = msgStyle.Render(wl)
				}
				if firstLine {
					sb.WriteString(fmt.Sprintf("%s%s %s  %s", prefix, timeStr, nameStr, content))
					firstLine = false
				} else {
					sb.WriteString(indentStr + content)
				}
				sb.WriteString("\n")
			}
		}
		currentLine += m.msgLineCount(msg)
	}

	return sb.String()
}

// ── Image support helpers ─────────────────────────────────────────────────────

// queueImageFetch returns a Cmd that downloads (and renders for inline backends)
// a single image URL. Returns nil if the image is already ready.
func (m *model) queueImageFetch(url string, artWidth int) tea.Cmd {
	if m.imgRenderer == nil || m.imgRenderer.backend == ImgBackendNone {
		return nil
	}
	if m.imgRenderer.backend == ImgBackendUeberzug {
		if m.imgRenderer.GetDownloaded(url) != "" {
			return nil
		}
		ir := m.imgRenderer
		return func() tea.Msg {
			ir.Download(url) //nolint
			return imageReadyMsg{url: url}
		}
	}
	// Inline backends: download + render with chafa
	if m.imgRenderer.GetRendered(url, artWidth) != nil {
		return nil
	}
	ir := m.imgRenderer
	return func() tea.Msg {
		localPath, err := ir.Download(url)
		if err != nil {
			return nil
		}
		lines, err := ir.RenderInline(localPath, artWidth)
		if err != nil {
			return nil
		}
		ir.SetRendered(url, artWidth, lines)
		return imageReadyMsg{url: url}
	}
}

// queuePendingImages queues fetch Cmds for all image messages not yet loaded.
func (m *model) queuePendingImages() []tea.Cmd {
	if m.imgRenderer == nil || m.imgRenderer.backend == ImgBackendNone {
		return nil
	}
	w := m.viewport.Width
	if w <= 0 {
		w = 80
	}
	var cmds []tea.Cmd
	seen := make(map[string]bool)
	for _, msg := range m.messages {
		if msg.IsDeleted || !isImageMessage(msg.MessageRaw) {
			continue
		}
		url := extractImageURL(msg.MessageRaw)
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		indent := 2 + 5 + 1 + len([]rune(msg.User.Username)) + 2
		artWidth := w - indent
		if artWidth < 10 {
			artWidth = 10
		}
		if cmd := m.queueImageFetch(url, artWidth); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return cmds
}

// updateImageOverlays sends draw/clear commands to Überzug++ for currently
// visible images. No-op for non-ueberzug backends.
func (m *model) updateImageOverlays() {
	if m.imgRenderer == nil || m.imgRenderer.backend != ImgBackendUeberzug || m.ueberzug == nil {
		return
	}

	vpY := m.viewport.YOffset
	vpH := m.viewport.Height
	imgH := m.imgRenderer.imgH
	const headerRows = 1 // one row for the top header bar

	// Build set of overlays that should be visible now
	wantVisible := make(map[string]bool)

	for i, pos := range m.imgPositions {
		screenLine := pos.vpLine - vpY
		// Skip images that start below the viewport or fully above it
		if screenLine >= vpH || screenLine+imgH <= 0 {
			continue
		}
		localPath := m.imgRenderer.GetDownloaded(pos.url)
		if localPath == "" {
			continue // not yet downloaded
		}
		// Terminal row where the art starts (0-indexed from top of terminal)
		termRow := headerRows + screenLine
		if termRow < headerRows {
			termRow = headerRows
		}
		// Clip height to viewport bottom
		dispH := imgH
		if screenLine+imgH > vpH {
			dispH = vpH - screenLine
		}
		if dispH <= 0 {
			continue
		}
		id := fmt.Sprintf("lolzimg_%d", i)
		wantVisible[id] = true
		m.ueberzug.drawIfChanged(id, localPath, pos.indent, termRow, pos.artWidth, dispH)
	}

	// Remove overlays that are no longer visible
	m.ueberzug.removeExcept(wantVisible)
}

// doCleanup stops Überzug++ and removes temp image files.
func (m *model) doCleanup() {
	if m.ueberzug != nil {
		m.ueberzug.clearAll()
		m.ueberzug.stop()
		m.ueberzug = nil
	}
	if m.imgRenderer != nil {
		m.imgRenderer.Cleanup()
	}
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	hyellow := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffd700")).Background(lipgloss.Color("#16213e"))
	hyellowDim := lipgloss.NewStyle().Foreground(lipgloss.Color("#ccaa00")).Background(lipgloss.Color("#16213e"))
	headerBg := lipgloss.NewStyle().Width(m.width).MaxWidth(m.width).Background(lipgloss.Color("#16213e")).Inline(true)

	dotColor := lipgloss.Color("#ff4757")
	if m.connected {
		dotColor = lipgloss.Color("#53d769")
	}
	connDot := lipgloss.NewStyle().Foreground(dotColor).Background(lipgloss.Color("#16213e")).Render("●")

	sep := hyellowDim.Render(" | ")
	headerParts := []string{
		hyellow.Render(" Chatbox-cli "),
		sep + hyellow.Render("lolz.live/gay1234"),
		sep + hyellow.Render(fmt.Sprintf("#%d %s", m.roomID, m.roomTitle)),
		sep + connDot,
		hyellowDim.Render(fmt.Sprintf(" [%d] ", m.msgCount)),
	}
	headerContent := ""
	for _, p := range headerParts {
		candidate := headerContent + p
		if lipgloss.Width(candidate) > m.width {
			break
		}
		headerContent = candidate
	}
	header := headerBg.Render(headerContent)

	vpStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(m.viewport.Height)
	chatArea := vpStyle.Render(m.viewport.View())

	errLine := ""
	if m.err != "" {
		errLine = errorStyle.Render("  ! " + m.err)
	}

	sendingIndicator := ""
	if m.sending {
		sendingIndicator = " ..."
	}

	var modeLine string
	var inputBox string

	switch m.mode {
	case modeEdit:
		modeLine = modeStyle.Render("  [EDIT] Редактирование (Esc — отмена)")
		inputBox = editBorder.Width(m.width - 4).Render(m.input.View() + sendingIndicator)
	case modeReply:
		modeLine = modeStyle.Render(fmt.Sprintf("  [REPLY] %s (Esc — отмена)", m.replyPreview))
		inputBox = replyBorder.Width(m.width - 4).Render(m.input.View() + sendingIndicator)
	case modeSelect:
		modeLine = modeStyle.Render("  [SELECT] ↑↓ выбрать · Enter — ответить · Esc — отмена")
		inputBox = replyBorder.Width(m.width - 4).Render(m.input.View())
	default:
		inputBox = inputBorder.Width(m.width - 4).Render(m.input.View() + sendingIndicator)
	}

	helpText := "  Enter: send | Ctrl+E: edit last | Tab: select·reply | Ctrl+R: quick reply | PgUp/Dn: scroll | Esc: exit"
	helpLines := wrapText(helpText, m.width-2)
	help := helpStyle.Render(strings.Join(helpLines, "\n"))

	parts := []string{header, chatArea}
	if errLine != "" {
		parts = append(parts, errLine)
	}
	if modeLine != "" {
		parts = append(parts, modeLine)
	}
	parts = append(parts, inputBox, help)

	out := lipgloss.JoinVertical(lipgloss.Left, parts...)
	// Prevent duplication on small terminals by capping output height
	outLines := strings.Split(out, "\n")
	if len(outLines) > m.height {
		outLines = outLines[:m.height]
	}
	return strings.Join(outLines, "\n")
}
