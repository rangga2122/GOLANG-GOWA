package server

import (
	"net/http"
	"strconv"
	"time"

	"gowa-broadcast/internal/broadcast"
	"gowa-broadcast/internal/database"
	"gowa-broadcast/internal/middleware"

	"github.com/gin-gonic/gin"
)

type CreateBroadcastListRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	CreatedBy   string `json:"created_by" binding:"required"`
}

type UpdateBroadcastListRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsActive    *bool  `json:"is_active"`
}

type AddRecipientsRequest struct {
	Recipients []RecipientRequest `json:"recipients" binding:"required"`
}

type RecipientRequest struct {
	JID         string `json:"jid" binding:"required"`
	Name        string `json:"name"`
	PhoneNumber string `json:"phone_number"`
}

type CreateScheduledMessageRequest struct {
	Name        string   `json:"name" binding:"required"`
	Recipients  []string `json:"recipients" binding:"required"`
	MessageType string   `json:"message_type" binding:"required"`
	Content     string   `json:"content" binding:"required"`
	MediaURL    string   `json:"media_url,omitempty"`
	ScheduledAt string   `json:"scheduled_at" binding:"required"` // RFC3339 format
	CronExpr    string   `json:"cron_expr,omitempty"`
	IsRecurring bool     `json:"is_recurring"`
}

// Broadcast List Handlers
func (s *Server) handleGetBroadcastLists(c *gin.Context) {
	// Get current user ID
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	var lists []database.BroadcastList
	query := s.db.Preload("Recipients").Where("user_id = ?", userID)

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset := (page - 1) * limit

	// Search
	if search := c.Query("search"); search != "" {
		query = query.Where("name LIKE ? OR description LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// Filter by active status
	if active := c.Query("active"); active != "" {
		if active == "true" {
			query = query.Where("is_active = ?", true)
		} else if active == "false" {
			query = query.Where("is_active = ?", false)
		}
	}

	var total int64
	query.Model(&database.BroadcastList{}).Count(&total)
	query.Offset(offset).Limit(limit).Find(&lists)

	c.JSON(200, gin.H{
		"broadcast_lists": lists,
		"total":          total,
		"page":           page,
		"limit":          limit,
	})
}

func (s *Server) handleCreateBroadcastList(c *gin.Context) {
	// Get current user ID
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	var req CreateBroadcastListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	broadcastList := &database.BroadcastList{
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		CreatedBy:   req.CreatedBy,
		IsActive:    true,
	}

	if err := s.db.Create(broadcastList).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to create broadcast list"})
		return
	}

	c.JSON(201, gin.H{
		"message":        "Broadcast list created successfully",
		"broadcast_list": broadcastList,
	})
}

func (s *Server) handleGetBroadcastList(c *gin.Context) {
	// Get current user ID
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid broadcast list ID"})
		return
	}

	var broadcastList database.BroadcastList
	if err := s.db.Preload("Recipients").Where("user_id = ?", userID).First(&broadcastList, uint(id)).Error; err != nil {
		c.JSON(404, gin.H{"error": "Broadcast list not found"})
		return
	}

	c.JSON(200, broadcastList)
}

func (s *Server) handleUpdateBroadcastList(c *gin.Context) {
	// Get current user ID
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid broadcast list ID"})
		return
	}

	var req UpdateBroadcastListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var broadcastList database.BroadcastList
	if err := s.db.Where("user_id = ?", userID).First(&broadcastList, uint(id)).Error; err != nil {
		c.JSON(404, gin.H{"error": "Broadcast list not found"})
		return
	}

	// Update fields
	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if err := s.db.Model(&broadcastList).Updates(updates).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to update broadcast list"})
		return
	}

	c.JSON(200, gin.H{
		"message":        "Broadcast list updated successfully",
		"broadcast_list": broadcastList,
	})
}

func (s *Server) handleDeleteBroadcastList(c *gin.Context) {
	// Get current user ID
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid broadcast list ID"})
		return
	}

	// Verify ownership before deletion
	var broadcastList database.BroadcastList
	if err := s.db.Where("user_id = ?", userID).First(&broadcastList, uint(id)).Error; err != nil {
		c.JSON(404, gin.H{"error": "Broadcast list not found"})
		return
	}

	// Delete recipients first
	s.db.Where("broadcast_list_id = ?", uint(id)).Delete(&database.BroadcastRecipient{})

	// Delete broadcast list
	if err := s.db.Delete(&database.BroadcastList{}, uint(id)).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to delete broadcast list"})
		return
	}

	c.JSON(200, gin.H{"message": "Broadcast list deleted successfully"})
}

func (s *Server) handleAddRecipients(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid broadcast list ID"})
		return
	}

	var req AddRecipientsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Check if broadcast list exists
	var broadcastList database.BroadcastList
	if err := s.db.First(&broadcastList, uint(id)).Error; err != nil {
		c.JSON(404, gin.H{"error": "Broadcast list not found"})
		return
	}

	// Add recipients
	var recipients []database.BroadcastRecipient
	for _, recipientReq := range req.Recipients {
		recipient := database.BroadcastRecipient{
			BroadcastListID: uint(id),
			JID:             recipientReq.JID,
			Name:            recipientReq.Name,
			PhoneNumber:     recipientReq.PhoneNumber,
			IsActive:        true,
		}
		recipients = append(recipients, recipient)
	}

	if err := s.db.Create(&recipients).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to add recipients"})
		return
	}

	c.JSON(201, gin.H{
		"message":    "Recipients added successfully",
		"recipients": recipients,
	})
}

func (s *Server) handleRemoveRecipient(c *gin.Context) {
	listID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid broadcast list ID"})
		return
	}

	recipientID, err := strconv.ParseUint(c.Param("recipientId"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid recipient ID"})
		return
	}

	if err := s.db.Where("id = ? AND broadcast_list_id = ?", uint(recipientID), uint(listID)).Delete(&database.BroadcastRecipient{}).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to remove recipient"})
		return
	}

	c.JSON(200, gin.H{"message": "Recipient removed successfully"})
}

// Broadcast Handlers
func (s *Server) handleCreateBroadcast(c *gin.Context) {
	var req broadcast.BroadcastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	resp, err := s.broadcastMgr.CreateBroadcast(&req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if resp.Success {
		c.JSON(201, resp)
	} else {
		c.JSON(400, resp)
	}
}

func (s *Server) handleGetBroadcastStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid broadcast ID"})
		return
	}

	status, err := s.broadcastMgr.GetBroadcastStatus(uint(id))
	if err != nil {
		c.JSON(404, gin.H{"error": "Broadcast not found"})
		return
	}

	c.JSON(200, status)
}

func (s *Server) handleCancelBroadcast(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid broadcast ID"})
		return
	}

	if err := s.broadcastMgr.CancelBroadcast(uint(id)); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Broadcast cancelled successfully"})
}

func (s *Server) handleGetActiveBroadcasts(c *gin.Context) {
	activeBroadcasts := s.broadcastMgr.ListActiveBroadcasts()
	c.JSON(200, gin.H{"active_broadcasts": activeBroadcasts})
}

func (s *Server) handleGetBroadcastHistory(c *gin.Context) {
	var broadcasts []database.BroadcastMessage
	query := s.db.Model(&database.BroadcastMessage{})

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset := (page - 1) * limit

	// Filter by status
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	// Filter by broadcast list
	if listID := c.Query("broadcast_list_id"); listID != "" {
		query = query.Where("broadcast_list_id = ?", listID)
	}

	var total int64
	query.Count(&total)
	query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&broadcasts)

	c.JSON(200, gin.H{
		"broadcasts": broadcasts,
		"total":      total,
		"page":       page,
		"limit":      limit,
	})
}

// Scheduled Message Handlers
func (s *Server) handleGetScheduledMessages(c *gin.Context) {
	// Get current user ID
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	var messages []database.ScheduledMessage
	query := s.db.Model(&database.ScheduledMessage{}).Where("user_id = ?", userID)

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset := (page - 1) * limit

	// Filter by status
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)
	query.Order("scheduled_at ASC").Offset(offset).Limit(limit).Find(&messages)

	c.JSON(200, gin.H{
		"scheduled_messages": messages,
		"total":             total,
		"page":              page,
		"limit":             limit,
	})
}

func (s *Server) handleCreateScheduledMessage(c *gin.Context) {
	// Get current user ID
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	var req CreateScheduledMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Parse scheduled time
	scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid scheduled_at format. Use RFC3339 format"})
		return
	}

	// Check if scheduled time is in the future
	if scheduledAt.Before(time.Now()) {
		c.JSON(400, gin.H{"error": "Scheduled time must be in the future"})
		return
	}

	// Convert recipients to JSON
	recipientsJSON, err := json.Marshal(req.Recipients)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to process recipients"})
		return
	}

	scheduledMsg := &database.ScheduledMessage{
		Name:        req.Name,
		Recipients:  string(recipientsJSON),
		MessageType: req.MessageType,
		Content:     req.Content,
		MediaURL:    req.MediaURL,
		ScheduledAt: scheduledAt,
		Status:      "pending",
		CronExpr:    req.CronExpr,
		IsRecurring: req.IsRecurring,
	}

	if err := s.db.Create(scheduledMsg).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to create scheduled message"})
		return
	}

	c.JSON(201, gin.H{
		"message":           "Scheduled message created successfully",
		"scheduled_message": scheduledMsg,
	})
}

func (s *Server) handleGetScheduledMessage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid scheduled message ID"})
		return
	}

	var scheduledMsg database.ScheduledMessage
	if err := s.db.First(&scheduledMsg, uint(id)).Error; err != nil {
		c.JSON(404, gin.H{"error": "Scheduled message not found"})
		return
	}

	c.JSON(200, scheduledMsg)
}

func (s *Server) handleUpdateScheduledMessage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid scheduled message ID"})
		return
	}

	var req CreateScheduledMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var scheduledMsg database.ScheduledMessage
	if err := s.db.First(&scheduledMsg, uint(id)).Error; err != nil {
		c.JSON(404, gin.H{"error": "Scheduled message not found"})
		return
	}

	// Check if message is still pending
	if scheduledMsg.Status != "pending" {
		c.JSON(400, gin.H{"error": "Cannot update non-pending scheduled message"})
		return
	}

	// Parse scheduled time
	scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid scheduled_at format. Use RFC3339 format"})
		return
	}

	// Convert recipients to JSON
	recipientsJSON, err := json.Marshal(req.Recipients)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to process recipients"})
		return
	}

	// Update fields
	scheduledMsg.Name = req.Name
	scheduledMsg.Recipients = string(recipientsJSON)
	scheduledMsg.MessageType = req.MessageType
	scheduledMsg.Content = req.Content
	scheduledMsg.MediaURL = req.MediaURL
	scheduledMsg.ScheduledAt = scheduledAt
	scheduledMsg.CronExpr = req.CronExpr
	scheduledMsg.IsRecurring = req.IsRecurring

	if err := s.db.Save(&scheduledMsg).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to update scheduled message"})
		return
	}

	c.JSON(200, gin.H{
		"message":           "Scheduled message updated successfully",
		"scheduled_message": scheduledMsg,
	})
}

func (s *Server) handleDeleteScheduledMessage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid scheduled message ID"})
		return
	}

	if err := s.db.Delete(&database.ScheduledMessage{}, uint(id)).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to delete scheduled message"})
		return
	}

	c.JSON(200, gin.H{"message": "Scheduled message deleted successfully"})
}