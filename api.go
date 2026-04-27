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
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
	IsMod    bool   `json:"is_moderator"`
	IsStaff  bool   `json:"is_staff"`
	IsBanned bool   `json:"is_banned"`
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
	url := fmt.Sprintf("%s/chatbox/messages?room_id=%d", c.baseURL, roomID)
	data, err := c.doRequest("GET", url, nil)
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
	bbGenericRe    = regexp.MustCompile(`\[/?[A-Za-z]+(?:=[^\]]*)?]`)
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

func isImageMessage(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	return strings.HasPrefix(trimmed, "[IMG]") && strings.HasSuffix(trimmed, "[/IMG]")
}

func extractImageURL(raw string) string {
	matches := bbImgRe.FindStringSubmatch(raw)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}
