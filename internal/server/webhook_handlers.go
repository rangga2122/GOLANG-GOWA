package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"gowa-broadcast/internal/database"

	"github.com/gin-gonic/gin"
)

type WebhookRequest struct {
	URL     string            `json:"url" binding:"required"`
	Secret  string            `json:"secret"`
	Events  []string          `json:"events"`
	Headers map[string]string `json:"headers"`
}

type WebhookResponse struct {
	ID        uint              `json:"id"`
	URL       string            `json:"url"`
	Secret    string            `json:"secret,omitempty"`
	Events    []string          `json:"events"`
	Headers   map[string]string `json:"headers"`
	Active    bool              `json:"active"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type WebhookLogResponse struct {
	ID           uint      `json:"id"`
	WebhookID    uint      `json:"webhook_id"`
	Event        string    `json:"event"`
	Payload      string    `json:"payload"`
	StatusCode   int       `json:"status_code"`
	ResponseBody string    `json:"response_body"`
	Error        string    `json:"error"`
	CreatedAt    time.Time `json:"created_at"`
}

type WebhookEvent struct {
	Event     string      `json:"event"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

type MessageWebhookData struct {
	MessageID string `json:"message_id"`
	FromJID   string `json:"from_jid"`
	ToJID     string `json:"to_jid"`
	Type      string `json:"type"`
	Content   string `json:"content"`
	IsFromMe  bool   `json:"is_from_me"`
	Timestamp int64  `json:"timestamp"`
}

type BroadcastWebhookData struct {
	BroadcastID     string `json:"broadcast_id"`
	Status          string `json:"status"`
	TotalRecipients int    `json:"total_recipients"`
	SentCount       int    `json:"sent_count"`
	FailedCount     int    `json:"failed_count"`
	Message         string `json:"message"`
}

func (s *Server) handleCreateWebhook(c *gin.Context) {
	var req WebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Validate events
	validEvents := map[string]bool{
		"message.received": true,
		"message.sent":     true,
		"broadcast.start":  true,
		"broadcast.end":    true,
		"connection":       true,
	}

	for _, event := range req.Events {
		if !validEvents[event] {
			c.JSON(400, gin.H{"error": fmt.Sprintf("Invalid event: %s", event)})
			return
		}
	}

	// Convert events to JSON
	eventsJSON, _ := json.Marshal(req.Events)
	headersJSON, _ := json.Marshal(req.Headers)

	webhook := database.Webhook{
		URL:     req.URL,
		Secret:  req.Secret,
		Events:  string(eventsJSON),
		Headers: string(headersJSON),
		Active:  true,
	}

	if err := s.db.Create(&webhook).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to create webhook"})
		return
	}

	response := WebhookResponse{
		ID:        webhook.ID,
		URL:       webhook.URL,
		Secret:    webhook.Secret,
		Events:    req.Events,
		Headers:   req.Headers,
		Active:    webhook.Active,
		CreatedAt: webhook.CreatedAt,
		UpdatedAt: webhook.UpdatedAt,
	}

	c.JSON(201, response)
}

func (s *Server) handleGetWebhooks(c *gin.Context) {
	var webhooks []database.Webhook
	if err := s.db.Find(&webhooks).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to get webhooks"})
		return
	}

	response := make([]WebhookResponse, len(webhooks))
	for i, webhook := range webhooks {
		var events []string
		var headers map[string]string
		json.Unmarshal([]byte(webhook.Events), &events)
		json.Unmarshal([]byte(webhook.Headers), &headers)

		response[i] = WebhookResponse{
			ID:        webhook.ID,
			URL:       webhook.URL,
			Events:    events,
			Headers:   headers,
			Active:    webhook.Active,
			CreatedAt: webhook.CreatedAt,
			UpdatedAt: webhook.UpdatedAt,
		}
	}

	c.JSON(200, response)
}

func (s *Server) handleGetWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid webhook ID"})
		return
	}

	var webhook database.Webhook
	if err := s.db.First(&webhook, uint(id)).Error; err != nil {
		c.JSON(404, gin.H{"error": "Webhook not found"})
		return
	}

	var events []string
	var headers map[string]string
	json.Unmarshal([]byte(webhook.Events), &events)
	json.Unmarshal([]byte(webhook.Headers), &headers)

	response := WebhookResponse{
		ID:        webhook.ID,
		URL:       webhook.URL,
		Secret:    webhook.Secret,
		Events:    events,
		Headers:   headers,
		Active:    webhook.Active,
		CreatedAt: webhook.CreatedAt,
		UpdatedAt: webhook.UpdatedAt,
	}

	c.JSON(200, response)
}

func (s *Server) handleUpdateWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid webhook ID"})
		return
	}

	var webhook database.Webhook
	if err := s.db.First(&webhook, uint(id)).Error; err != nil {
		c.JSON(404, gin.H{"error": "Webhook not found"})
		return
	}

	var req WebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Validate events
	validEvents := map[string]bool{
		"message.received": true,
		"message.sent":     true,
		"broadcast.start":  true,
		"broadcast.end":    true,
		"connection":       true,
	}

	for _, event := range req.Events {
		if !validEvents[event] {
			c.JSON(400, gin.H{"error": fmt.Sprintf("Invalid event: %s", event)})
			return
		}
	}

	// Convert events to JSON
	eventsJSON, _ := json.Marshal(req.Events)
	headersJSON, _ := json.Marshal(req.Headers)

	webhook.URL = req.URL
	webhook.Secret = req.Secret
	webhook.Events = string(eventsJSON)
	webhook.Headers = string(headersJSON)

	if err := s.db.Save(&webhook).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to update webhook"})
		return
	}

	response := WebhookResponse{
		ID:        webhook.ID,
		URL:       webhook.URL,
		Secret:    webhook.Secret,
		Events:    req.Events,
		Headers:   req.Headers,
		Active:    webhook.Active,
		CreatedAt: webhook.CreatedAt,
		UpdatedAt: webhook.UpdatedAt,
	}

	c.JSON(200, response)
}

func (s *Server) handleDeleteWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid webhook ID"})
		return
	}

	if err := s.db.Delete(&database.Webhook{}, uint(id)).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to delete webhook"})
		return
	}

	c.JSON(200, gin.H{"message": "Webhook deleted successfully"})
}

func (s *Server) handleToggleWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid webhook ID"})
		return
	}

	var webhook database.Webhook
	if err := s.db.First(&webhook, uint(id)).Error; err != nil {
		c.JSON(404, gin.H{"error": "Webhook not found"})
		return
	}

	webhook.Active = !webhook.Active
	if err := s.db.Save(&webhook).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to toggle webhook"})
		return
	}

	c.JSON(200, gin.H{"active": webhook.Active})
}

func (s *Server) handleGetWebhookLogs(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid webhook ID"})
		return
	}

	// Parse query parameters
	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := 0
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	var logs []database.WebhookLog
	query := s.db.Where("webhook_id = ?", uint(id)).Order("created_at DESC")
	if err := query.Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to get webhook logs"})
		return
	}

	// Get total count
	var total int64
	s.db.Model(&database.WebhookLog{}).Where("webhook_id = ?", uint(id)).Count(&total)

	response := make([]WebhookLogResponse, len(logs))
	for i, log := range logs {
		response[i] = WebhookLogResponse{
			ID:           log.ID,
			WebhookID:    log.WebhookID,
			Event:        log.Event,
			Payload:      log.Payload,
			StatusCode:   log.StatusCode,
			ResponseBody: log.ResponseBody,
			Error:        log.Error,
			CreatedAt:    log.CreatedAt,
		}
	}

	c.JSON(200, gin.H{
		"logs":   response,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// SendWebhook sends webhook event to all active webhooks
func (s *Server) SendWebhook(event string, data interface{}) {
	var webhooks []database.Webhook
	if err := s.db.Where("active = ?", true).Find(&webhooks).Error; err != nil {
		return
	}

	webhookEvent := WebhookEvent{
		Event:     event,
		Timestamp: time.Now(),
		Data:      data,
	}

	payload, err := json.Marshal(webhookEvent)
	if err != nil {
		return
	}

	for _, webhook := range webhooks {
		// Check if webhook is subscribed to this event
		var events []string
		if err := json.Unmarshal([]byte(webhook.Events), &events); err != nil {
			continue
		}

		subscribed := false
		for _, e := range events {
			if e == event {
				subscribed = true
				break
			}
		}

		if !subscribed {
			continue
		}

		// Send webhook in goroutine
		go s.sendWebhookRequest(webhook, string(payload), event)
	}
}

func (s *Server) sendWebhookRequest(webhook database.Webhook, payload, event string) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("POST", webhook.URL, bytes.NewBufferString(payload))
	if err != nil {
		s.logWebhookError(webhook.ID, event, payload, 0, "", err.Error())
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GOWA-Broadcast-Webhook/1.0")

	// Add custom headers
	var headers map[string]string
	if err := json.Unmarshal([]byte(webhook.Headers), &headers); err == nil {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}

	// Add signature if secret is provided
	if webhook.Secret != "" {
		// You can implement HMAC signature here
		// For now, just add as header
		req.Header.Set("X-Webhook-Secret", webhook.Secret)
	}

	resp, err := client.Do(req)
	if err != nil {
		s.logWebhookError(webhook.ID, event, payload, 0, "", err.Error())
		return
	}
	defer resp.Body.Close()

	// Read response body
	respBody := make([]byte, 1024) // Limit response body size
	n, _ := resp.Body.Read(respBody)
	respBodyStr := string(respBody[:n])

	// Log webhook result
	log := database.WebhookLog{
		WebhookID:    webhook.ID,
		Event:        event,
		Payload:      payload,
		StatusCode:   resp.StatusCode,
		ResponseBody: respBodyStr,
	}

	if resp.StatusCode >= 400 {
		log.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	s.db.Create(&log)
}

func (s *Server) logWebhookError(webhookID uint, event, payload string, statusCode int, responseBody, errorMsg string) {
	log := database.WebhookLog{
		WebhookID:    webhookID,
		Event:        event,
		Payload:      payload,
		StatusCode:   statusCode,
		ResponseBody: responseBody,
		Error:        errorMsg,
	}
	s.db.Create(&log)
}