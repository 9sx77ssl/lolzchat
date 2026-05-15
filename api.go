package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type APIClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type ChatMessage struct {
	MessageID  int          `json:"message_id"`
	Date       int64        `json:"date"`
	Message    string       `json:"message"`
	MessageRaw string       `json:"messageRaw"`
	IsDeleted  bool         `json:"is_deleted"`
	User       ChatUser     `json:"user"`
	Room       ChatRoom     `json:"room"`
	Reply      *ChatMessage `json:"reply_message,omitempty"`
}

type ChatUser struct {
	UserID             int    `json:"user_id"`
	Username           string `json:"username"`
	UserTitle          string `json:"user_title"`
	IsAdmin            bool   `json:"is_admin"`
	IsMod              bool   `json:"is_moderator"`
	IsStaff            bool   `json:"is_staff"`
	IsSuperAdmin       bool   `json:"is_super_admin"`
	IsBanned           bool   `json:"is_banned"`
	DisplayIconGroupID int    `json:"display_icon_group_id"`
	UniqUsernameCss    string `json:"uniq_username_css"`
	Rendered           struct {
		Username string `json:"username"`
	} `json:"rendered"`
}

type ChatRoom struct {
	RoomID int    `json:"room_id"`
	Title  string `json:"title"`
}

type MessagesResponse struct {
	Messages []ChatMessage `json:"messages"`
}

type PostMessageResponse struct {
	Message ChatMessage `json:"message"`
}

type MeResponse struct {
	User struct {
		UserID    int    `json:"user_id"`
		Username  string `json:"username"`
		ShortLink string `json:"short_link"`
	} `json:"user"`
}

type RoomInfo struct {
	RoomID int    `json:"room_id"`
	Title  string `json:"title"`
	Eng    bool   `json:"eng"`
	Market bool   `json:"market"`
}

type RoomsResponse struct {
	Rooms []RoomInfo `json:"rooms"`
}

func NewAPIClient(baseURL, token string) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *APIClient) doRequest(method, url string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(data))
	}

	return data, nil
}

func (c *APIClient) GetMe() (int, string, string, error) {
	data, err := c.doRequest("GET", c.baseURL+"/users/me", nil)
	if err != nil {
		return 0, "", "", err
	}
	var resp MeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, "", "", err
	}
	link := resp.User.ShortLink
	if link == "" {
		link = resp.User.Username
	}
	return resp.User.UserID, resp.User.Username, link, nil
}

func (c *APIClient) GetRooms() ([]RoomInfo, error) {
	data, err := c.doRequest("GET", c.baseURL+"/chatbox", nil)
	if err != nil {
		return nil, err
	}
	var resp RoomsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Rooms, nil
}

func (c *APIClient) GetMessages(roomID int) ([]ChatMessage, error) {
	return c.GetMessagesBefore(roomID, 0)
}

func (c *APIClient) GetMessagesBefore(roomID, beforeID int) ([]ChatMessage, error) {
	u := fmt.Sprintf("%s/chatbox/messages?room_id=%d", c.baseURL, roomID)
	if beforeID > 0 {
		u += fmt.Sprintf("&before_message_id=%d", beforeID)
	}
	data, err := c.doRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	var resp MessagesResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Messages, nil
}

func (c *APIClient) SendMessage(roomID int, message string) (*ChatMessage, error) {
	payload := map[string]interface{}{
		"room_id": roomID,
		"message": message,
	}
	body, _ := json.Marshal(payload)
	data, err := c.doRequest("POST", c.baseURL+"/chatbox/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	var resp PostMessageResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp.Message, nil
}

func (c *APIClient) EditMessage(messageID int, message string) (*ChatMessage, error) {
	payload := map[string]interface{}{
		"message_id": messageID,
		"message":    message,
	}
	body, _ := json.Marshal(payload)
	data, err := c.doRequest("PUT", c.baseURL+"/chatbox/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	var resp PostMessageResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp.Message, nil
}

func (c *APIClient) ReplyMessage(roomID int, replyMessageID int, message string) (*ChatMessage, error) {
	payload := map[string]interface{}{
		"room_id":          roomID,
		"reply_message_id": replyMessageID,
		"message":          message,
	}
	body, _ := json.Marshal(payload)
	data, err := c.doRequest("POST", c.baseURL+"/chatbox/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	var resp PostMessageResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp.Message, nil
}

func (c *APIClient) DeleteMessage(messageID int) error {
	payload := map[string]interface{}{
		"message_id": messageID,
	}
	body, _ := json.Marshal(payload)
	_, err := c.doRequest("DELETE", c.baseURL+"/chatbox/messages", bytes.NewReader(body))
	return err
}

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)
var htmlEntityMap = map[string]string{
	"&amp;":  "&",
	"&lt;":   "<",
	"&gt;":   ">",
	"&quot;": "\"",
	"&#39;":  "'",
	"&nbsp;": " ",
}

var (
	bbUserRe       = regexp.MustCompile(`\[USER=\d+\]@?`)
	bbUserEndRe    = regexp.MustCompile(`\[/USER\]`)
	bbTooltipRe    = regexp.MustCompile(`\[tooltip=\d+\]`)
	bbTooltipEndRe = regexp.MustCompile(`\[/tooltip\]`)
	bbImgRe        = regexp.MustCompile(`\[IMG\](.*?)\[/IMG\]`)
	bbURLRe        = regexp.MustCompile(`(?i)\[URL(?:=[^\]]*)?\](.*?)\[/URL\]`)
	bbGenericRe    = regexp.MustCompile(`\[/?[A-Za-z]+(?:=[^\]]*)?]`)
	// Bare image URL — ends with image extension
	imgExtRe = regexp.MustCompile(`(?i)\.(jpe?g|png|gif|webp|bmp|svg|tiff?|ico|avif|heic)(\?[^\s]*)?$`)
	// Known image CDN domains — URL from these is always an image
	imgCDNPatterns = []string{
		"nztcdn.com/files/",
		"i.pinimg.com/",
		"i.imgur.com/",
		"cdn.discordapp.com/attachments/",
		"media.discordapp.net/attachments/",
		"pbs.twimg.com/media/",
		"sun9-", // vk cdn
	}
	// Extract any URL from text
	anyURLRe = regexp.MustCompile(`https?://[^\s\[\]<>"]+`)
)

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, "")
	for entity, char := range htmlEntityMap {
		s = strings.ReplaceAll(s, entity, char)
	}
	return strings.TrimSpace(s)
}

func cleanBBCode(s string) string {
	s = bbImgRe.ReplaceAllString(s, "$1")
	s = bbUserRe.ReplaceAllString(s, "")
	s = bbUserEndRe.ReplaceAllString(s, "")
	s = bbTooltipRe.ReplaceAllString(s, "")
	s = bbTooltipEndRe.ReplaceAllString(s, "")
	s = bbGenericRe.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	return s
}

func cleanMessage(raw string, html string) string {
	// Pure image message — return marker instead of URL
	if isImageMessage(raw) {
		return "[изображение]"
	}
	text := raw
	if text == "" {
		text = stripHTML(html)
	}
	result := cleanBBCode(text)
	if result == "" {
		if strings.Contains(raw, "[IMG]") || strings.Contains(html, "<img") {
			return "[изображение]"
		}
	}
	return result
}

// extractRawURL extracts the meaningful URL from a message raw string,
// stripping BB-code [IMG], [URL] wrappers.
func extractRawURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	// [IMG]url[/IMG]
	if m := bbImgRe.FindStringSubmatch(trimmed); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	// [URL=...]text[/URL] or [URL]url[/URL]
	if m := bbURLRe.FindStringSubmatch(trimmed); len(m) > 1 {
		inner := strings.TrimSpace(m[1])
		if strings.HasPrefix(inner, "http") {
			return inner
		}
	}
	// Any URL in the text
	if m := anyURLRe.FindString(trimmed); m != "" {
		return m
	}
	return ""
}

// looksLikeImageURL checks if a URL looks like it points to an image.
func looksLikeImageURL(url string) bool {
	if url == "" {
		return false
	}
	lower := strings.ToLower(url)
	// Extension-based
	if imgExtRe.MatchString(url) {
		return true
	}
	// CDN domain-based
	for _, pat := range imgCDNPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}

// isImageMessage returns true if the entire message is just an image.
func isImageMessage(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	// [IMG]...[/IMG]
	if strings.HasPrefix(trimmed, "[IMG]") && strings.HasSuffix(trimmed, "[/IMG]") {
		return true
	}
	// Message is just a single URL that looks like an image
	url := extractRawURL(raw)
	if url == "" {
		return false
	}
	// Make sure the entire message content IS the URL (not a URL within text)
	stripped := bbURLRe.ReplaceAllString(trimmed, "$1")
	stripped = bbImgRe.ReplaceAllString(stripped, "$1")
	stripped = strings.TrimSpace(stripped)
	// If after stripping, the content is just the URL — it's an image message
	if stripped == url || trimmed == url {
		return looksLikeImageURL(url)
	}
	// Also handle [URL=X]X[/URL] where the visible text equals the URL
	if looksLikeImageURL(url) {
		noTags := bbGenericRe.ReplaceAllString(trimmed, "")
		noTags = strings.TrimSpace(noTags)
		if noTags == url {
			return true
		}
	}
	return false
}

func extractImageURL(raw string) string {
	url := extractRawURL(raw)
	if url != "" && looksLikeImageURL(url) {
		return url
	}
	return ""
}
