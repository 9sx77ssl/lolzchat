package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// ImgBackend describes how images are rendered in the terminal.
type ImgBackend int

const (
	ImgBackendNone     ImgBackend = iota // URL text only
	ImgBackendChafa                      // inline ANSI block-art via chafa
	ImgBackendKitty                      // inline kitty graphics via chafa --format=kitty
	ImgBackendUeberzug                   // Überzug++ pixel overlay
)

const defaultImgHeight = 5

// imgViewPos tracks where image art appears inside the viewport content string.
type imgViewPos struct {
	msgID    int
	url      string
	vpLine   int // first ART line in the full viewport content (0-indexed)
	artWidth int // width the art was/will be rendered at
	indent   int // column where art starts (for overlay x)
}

type renderKey struct {
	url   string
	width int
}

// ImageRenderer manages image downloading, caching, and terminal rendering.
type ImageRenderer struct {
	backend    ImgBackend
	imgH       int
	tempDir    string
	mu         sync.Mutex
	downloaded map[string]string      // url → local file path
	rendered   map[renderKey][]string // (url,width) → ANSI lines
}

func newImageRenderer(mode string, imgH int) *ImageRenderer {
	if imgH <= 0 {
		imgH = defaultImgHeight
	}
	ir := &ImageRenderer{
		imgH:       imgH,
		downloaded: make(map[string]string),
		rendered:   make(map[renderKey][]string),
	}
	var err error
	ir.tempDir, err = os.MkdirTemp("", "lolzchat-img-*")
	if err != nil {
		ir.tempDir = filepath.Join(os.TempDir(), "lolzchat-img")
		os.MkdirAll(ir.tempDir, 0700) //nolint
	}
	ir.backend = ir.detectBackend(mode)
	return ir
}

func (ir *ImageRenderer) detectBackend(mode string) ImgBackend {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "none":
		return ImgBackendNone
	case "chafa":
		if _, e := exec.LookPath("chafa"); e == nil {
			return ImgBackendChafa
		}
		return ImgBackendNone
	case "kitty":
		if _, e := exec.LookPath("chafa"); e == nil {
			return ImgBackendKitty
		}
		return ImgBackendNone
	case "ueberzug", "ueberzugpp":
		if _, e := exec.LookPath("ueberzugpp"); e == nil {
			return ImgBackendUeberzug
		}
		return ImgBackendNone
	}
	// auto-detect: chafa (inline, bordered) > kitty > none
	// ueberzug is only used if explicitly set (overlays can't be bordered)
	if _, e := exec.LookPath("chafa"); e == nil {
		if os.Getenv("KITTY_WINDOW_ID") != "" || os.Getenv("TERM") == "xterm-kitty" {
			return ImgBackendKitty
		}
		return ImgBackendChafa
	}
	return ImgBackendNone
}

func (ir *ImageRenderer) urlLocalPath(url string) string {
	h := md5.Sum([]byte(url))
	base := strings.Split(url, "?")[0]
	ext := filepath.Ext(base)
	if ext == "" || len(ext) > 6 {
		ext = ".webp"
	}
	return filepath.Join(ir.tempDir, fmt.Sprintf("%x%s", h, ext))
}

// GetDownloaded returns the local file path if already downloaded, or "".
func (ir *ImageRenderer) GetDownloaded(url string) string {
	ir.mu.Lock()
	defer ir.mu.Unlock()
	return ir.downloaded[url]
}

// Download fetches url to local storage and caches the path. Idempotent.
func (ir *ImageRenderer) Download(url string) (string, error) {
	ir.mu.Lock()
	if p := ir.downloaded[url]; p != "" {
		ir.mu.Unlock()
		return p, nil
	}
	ir.mu.Unlock()

	path := ir.urlLocalPath(url)
	if _, err := os.Stat(path); err != nil {
		resp, err := http.Get(url) //nolint:noctx
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return "", err
		}
	}

	ir.mu.Lock()
	ir.downloaded[url] = path
	ir.mu.Unlock()
	return path, nil
}

// GetRendered returns cached ANSI lines for (url, width), or nil if not ready.
func (ir *ImageRenderer) GetRendered(url string, width int) []string {
	ir.mu.Lock()
	defer ir.mu.Unlock()
	return ir.rendered[renderKey{url, width}]
}

// SetRendered stores ANSI lines for (url, width).
func (ir *ImageRenderer) SetRendered(url string, width int, lines []string) {
	ir.mu.Lock()
	defer ir.mu.Unlock()
	ir.rendered[renderKey{url, width}] = lines
}

// InvalidateRenderCache clears the render cache (e.g. on terminal resize).
func (ir *ImageRenderer) InvalidateRenderCache() {
	ir.mu.Lock()
	defer ir.mu.Unlock()
	ir.rendered = make(map[renderKey][]string)
}

// RenderInline runs chafa on localPath and returns exactly imgH ANSI lines.
// The width parameter is the TOTAL available width (including border).
// Art is rendered at width-2 to leave room for │ borders.
func (ir *ImageRenderer) RenderInline(localPath string, width int) ([]string, error) {
	innerW := width - 2
	if innerW < 4 {
		innerW = 4
	}
	sizeStr := fmt.Sprintf("%dx%d", innerW, ir.imgH)
	var args []string
	if ir.backend == ImgBackendKitty {
		args = []string{"--size", sizeStr, "--format=kitty", "--stretch", localPath}
	} else {
		args = []string{"--size", sizeStr, "--colors=full", "--stretch", localPath}
	}
	out, err := exec.Command("chafa", args...).Output()
	if err != nil {
		return nil, err
	}
	raw := strings.TrimRight(string(out), "\n")
	lines := strings.Split(raw, "\n")
	for len(lines) < ir.imgH {
		lines = append(lines, "")
	}
	return lines[:ir.imgH], nil
}

// Placeholder returns imgH+2 lines (border top + empty/loading + border bottom).
func (ir *ImageRenderer) Placeholder(url string, width int) []string {
	if width < 6 {
		width = 6
	}
	inner := width - 2 // width between │ borders
	hline := strings.Repeat("─", inner)
	empty := "│" + strings.Repeat(" ", inner) + "│"

	lines := make([]string, 0, ir.imgH+2)
	lines = append(lines, "╭"+hline+"╮")

	// First inner line shows "загрузка..." label
	maxL := inner - 2
	if maxL < 0 {
		maxL = 0
	}
	label := "⏳ загрузка..."
	if runes := []rune(label); len(runes) > maxL {
		if maxL > 1 {
			label = string(runes[:maxL-1]) + "…"
		} else {
			label = ""
		}
	}
	lines = append(lines, fmt.Sprintf("│ %-*s│", inner-1, label))

	for i := 1; i < ir.imgH; i++ {
		lines = append(lines, empty)
	}
	lines = append(lines, "╰"+hline+"╯")
	return lines
}

// BorderWrap wraps art lines in a box-drawing border. Returns imgH+2 lines.
func (ir *ImageRenderer) BorderWrap(artLines []string, width int) []string {
	if width < 6 {
		width = 6
	}
	inner := width - 2
	hline := strings.Repeat("─", inner)

	lines := make([]string, 0, len(artLines)+2)
	lines = append(lines, "╭"+hline+"╮")
	for _, al := range artLines {
		// Pad or trim art line to fit exactly inside border
		visLen := lipglossWidth(al)
		if visLen < inner {
			al += strings.Repeat(" ", inner-visLen)
		}
		lines = append(lines, "│"+al+"│")
	}
	lines = append(lines, "╰"+hline+"╯")
	return lines
}

// lipglossWidth returns the visible width of a string (ignoring ANSI escapes).
func lipglossWidth(s string) int {
	// Strip ANSI escape sequences to measure visible width
	inEsc := false
	w := 0
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '~' {
				inEsc = false
			}
			continue
		}
		w++
	}
	return w
}

// Cleanup removes the temporary image directory.
func (ir *ImageRenderer) Cleanup() {
	if ir.tempDir != "" && strings.Contains(ir.tempDir, "lolzchat-img") {
		os.RemoveAll(ir.tempDir)
	}
}

// ── Überzug++ manager ────────────────────────────────────────────────────────

type ueberzugPayload struct {
	Action     string `json:"action"`
	Identifier string `json:"identifier"`
	Path       string `json:"path,omitempty"`
	X          int    `json:"x,omitempty"`
	Y          int    `json:"y,omitempty"`
	MaxWidth   int    `json:"max_width,omitempty"`
	MaxHeight  int    `json:"max_height,omitempty"`
}

type overlayState struct {
	path string
	x, y int
	w, h int
}

// UeberzugManager wraps an ueberzugpp layer subprocess and sends draw commands via its stdin.
type UeberzugManager struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	mu     sync.Mutex
	active map[string]overlayState
}

func newUeberzugManager() (*UeberzugManager, error) {
	cmd := exec.Command("ueberzugpp", "layer", "--silent")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &UeberzugManager{
		cmd:    cmd,
		stdin:  stdin,
		active: make(map[string]overlayState),
	}, nil
}

func (u *UeberzugManager) draw(id, path string, x, y, w, h int) {
	u.mu.Lock()
	defer u.mu.Unlock()
	p := ueberzugPayload{
		Action: "add", Identifier: id,
		Path: path, X: x, Y: y, MaxWidth: w, MaxHeight: h,
	}
	data, _ := json.Marshal(p)
	fmt.Fprintf(u.stdin, "%s\n", data)
	u.active[id] = overlayState{path: path, x: x, y: y, w: w, h: h}
}

// drawIfChanged only sends a draw command if the overlay position/size actually changed.
func (u *UeberzugManager) drawIfChanged(id, path string, x, y, w, h int) {
	u.mu.Lock()
	prev, exists := u.active[id]
	u.mu.Unlock()
	if exists && prev.path == path && prev.x == x && prev.y == y && prev.w == w && prev.h == h {
		return // nothing changed
	}
	u.draw(id, path, x, y, w, h)
}

func (u *UeberzugManager) remove(id string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	p := ueberzugPayload{Action: "remove", Identifier: id}
	data, _ := json.Marshal(p)
	fmt.Fprintf(u.stdin, "%s\n", data)
	delete(u.active, id)
}

// removeExcept removes all overlays whose id is NOT in the keep set.
func (u *UeberzugManager) removeExcept(keep map[string]bool) {
	u.mu.Lock()
	ids := make([]string, 0, len(u.active))
	for id := range u.active {
		if !keep[id] {
			ids = append(ids, id)
		}
	}
	u.mu.Unlock()
	for _, id := range ids {
		u.remove(id)
	}
}

func (u *UeberzugManager) clearAll() {
	u.mu.Lock()
	ids := make([]string, 0, len(u.active))
	for id := range u.active {
		ids = append(ids, id)
	}
	u.mu.Unlock()
	for _, id := range ids {
		u.remove(id)
	}
}

func (u *UeberzugManager) stop() {
	if u.stdin != nil {
		u.stdin.Close()
	}
	if u.cmd != nil && u.cmd.Process != nil {
		u.cmd.Process.Kill()
		u.cmd.Wait() //nolint
	}
}

// initRenderer creates an ImageRenderer and, if needed, starts UeberzugManager.
// Falls back to chafa/none if ueberzugpp fails to start.
func initRenderer(mode string, imgH int) (*ImageRenderer, *UeberzugManager) {
	ir := newImageRenderer(mode, imgH)
	if ir.backend != ImgBackendUeberzug {
		return ir, nil
	}
	uz, err := newUeberzugManager()
	if err != nil {
		// Ueberzug++ couldn't start — fall back gracefully
		if _, e := exec.LookPath("chafa"); e == nil {
			ir.backend = ImgBackendChafa
		} else {
			ir.backend = ImgBackendNone
		}
		return ir, nil
	}
	return ir, uz
}
