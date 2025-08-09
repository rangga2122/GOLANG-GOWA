package broadcast

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"gowa-broadcast/internal/config"
	"gowa-broadcast/internal/database"
	"gowa-broadcast/internal/whatsapp"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type Manager struct {
	cfg      *config.Config
	db       *gorm.DB
	waClient *whatsapp.Client
	mu       sync.RWMutex
	active   map[uint]*BroadcastJob
}

type BroadcastJob struct {
	ID              uint
	BroadcastListID uint
	MessageType     string
	Content         string
	MediaURL        string
	Recipients      []string
	Status          string
	SentCount       int
	FailedCount     int
	TotalRecipients int
	StartedAt       *time.Time
	CompletedAt     *time.Time
	cancel          chan bool
}

type BroadcastRequest struct {
	BroadcastListID uint   `json:"broadcast_list_id" binding:"required"`
	MessageType     string `json:"message_type" binding:"required"` // text, image, document, audio, video
	Content         string `json:"content" binding:"required"`
	MediaURL        string `json:"media_url,omitempty"`
	ScheduledAt     string `json:"scheduled_at,omitempty"` // RFC3339 format
}

type BroadcastResponse struct {
	Success       bool   `json:"success"`
	BroadcastID   uint   `json:"broadcast_id,omitempty"`
	Message       string `json:"message"`
	TotalRecipients int  `json:"total_recipients,omitempty"`
	EstimatedTime string `json:"estimated_time,omitempty"`
}

type BroadcastStatus struct {
	ID              uint       `json:"id"`
	BroadcastListID uint       `json:"broadcast_list_id"`
	Status          string     `json:"status"`
	SentCount       int        `json:"sent_count"`
	FailedCount     int        `json:"failed_count"`
	TotalRecipients int        `json:"total_recipients"`
	Progress        float64    `json:"progress"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

func NewManager(cfg *config.Config, db *gorm.DB, waClient *whatsapp.Client) *Manager {
	return &Manager{
		cfg:      cfg,
		db:       db,
		waClient: waClient,
		active:   make(map[uint]*BroadcastJob),
	}
}

// CreateBroadcast creates a new broadcast
func (m *Manager) CreateBroadcast(req *BroadcastRequest) (*BroadcastResponse, error) {
	// Validate broadcast list
	var broadcastList database.BroadcastList
	if err := m.db.Preload("Recipients").First(&broadcastList, req.BroadcastListID).Error; err != nil {
		return &BroadcastResponse{
			Success: false,
			Message: "Broadcast list not found",
		}, err
	}

	if !broadcastList.IsActive {
		return &BroadcastResponse{
			Success: false,
			Message: "Broadcast list is not active",
		}, fmt.Errorf("broadcast list is not active")
	}

	// Get active recipients
	activeRecipients := make([]database.BroadcastRecipient, 0)
	for _, recipient := range broadcastList.Recipients {
		if recipient.IsActive {
			activeRecipients = append(activeRecipients, recipient)
		}
	}

	if len(activeRecipients) == 0 {
		return &BroadcastResponse{
			Success: false,
			Message: "No active recipients found",
		}, fmt.Errorf("no active recipients")
	}

	// Check recipient limit
	if len(activeRecipients) > m.cfg.Broadcast.MaxRecipients {
		return &BroadcastResponse{
			Success: false,
			Message: fmt.Sprintf("Too many recipients. Maximum allowed: %d", m.cfg.Broadcast.MaxRecipients),
		}, fmt.Errorf("too many recipients")
	}

	// Create broadcast message record
	broadcastMsg := &database.BroadcastMessage{
		BroadcastListID: req.BroadcastListID,
		MessageType:     req.MessageType,
		Content:         req.Content,
		MediaURL:        req.MediaURL,
		Status:          "pending",
		SentCount:       0,
		FailedCount:     0,
		TotalRecipients: len(activeRecipients),
	}

	if err := m.db.Create(broadcastMsg).Error; err != nil {
		return &BroadcastResponse{
			Success: false,
			Message: "Failed to create broadcast",
		}, err
	}

	// Calculate estimated time
	delayMs := time.Duration(m.cfg.Broadcast.DelayMS) * time.Millisecond
	estimatedTime := time.Duration(len(activeRecipients)) * delayMs

	// Start broadcast if not scheduled
	if req.ScheduledAt == "" {
		go m.executeBroadcast(broadcastMsg.ID, activeRecipients)
	} else {
		// TODO: Implement scheduled broadcast
		logrus.Info("Scheduled broadcast not implemented yet")
	}

	return &BroadcastResponse{
		Success:         true,
		BroadcastID:     broadcastMsg.ID,
		Message:         "Broadcast created successfully",
		TotalRecipients: len(activeRecipients),
		EstimatedTime:   estimatedTime.String(),
	}, nil
}

// executeBroadcast executes the broadcast
func (m *Manager) executeBroadcast(broadcastID uint, recipients []database.BroadcastRecipient) {
	logrus.Infof("Starting broadcast %d with %d recipients", broadcastID, len(recipients))

	// Get broadcast message
	var broadcastMsg database.BroadcastMessage
	if err := m.db.First(&broadcastMsg, broadcastID).Error; err != nil {
		logrus.Errorf("Failed to get broadcast message: %v", err)
		return
	}

	// Update status to sending
	now := time.Now()
	broadcastMsg.Status = "sending"
	broadcastMsg.StartedAt = &now
	m.db.Save(&broadcastMsg)

	// Create job
	job := &BroadcastJob{
		ID:              broadcastMsg.ID,
		BroadcastListID: broadcastMsg.BroadcastListID,
		MessageType:     broadcastMsg.MessageType,
		Content:         broadcastMsg.Content,
		MediaURL:        broadcastMsg.MediaURL,
		Recipients:      make([]string, len(recipients)),
		Status:          "sending",
		SentCount:       0,
		FailedCount:     0,
		TotalRecipients: len(recipients),
		StartedAt:       &now,
		cancel:          make(chan bool, 1),
	}

	// Convert recipients to JIDs
	for i, recipient := range recipients {
		job.Recipients[i] = recipient.JID
	}

	// Add to active jobs
	m.mu.Lock()
	m.active[broadcastID] = job
	m.mu.Unlock()

	// Execute broadcast
	m.sendToRecipients(job)

	// Remove from active jobs
	m.mu.Lock()
	delete(m.active, broadcastID)
	m.mu.Unlock()

	// Update final status
	completedAt := time.Now()
	broadcastMsg.Status = "completed"
	broadcastMsg.SentCount = job.SentCount
	broadcastMsg.FailedCount = job.FailedCount
	broadcastMsg.CompletedAt = &completedAt
	m.db.Save(&broadcastMsg)

	logrus.Infof("Broadcast %d completed. Sent: %d, Failed: %d", broadcastID, job.SentCount, job.FailedCount)
}

// sendToRecipients sends messages to all recipients
func (m *Manager) sendToRecipients(job *BroadcastJob) {
	delayMs := time.Duration(m.cfg.Broadcast.DelayMS) * time.Millisecond
	rateLimit := m.cfg.Broadcast.RateLimit
	sentInWindow := 0
	windowStart := time.Now()

	for i, recipientJID := range job.Recipients {
		// Check for cancellation
		select {
		case <-job.cancel:
			logrus.Infof("Broadcast %d cancelled", job.ID)
			return
		default:
		}

		// Rate limiting
		if sentInWindow >= rateLimit {
			// Wait for next window
			elapsed := time.Since(windowStart)
			if elapsed < time.Minute {
				time.Sleep(time.Minute - elapsed)
			}
			sentInWindow = 0
			windowStart = time.Now()
		}

		// Send message
		var err error
		switch job.MessageType {
		case "text":
			_, err = m.waClient.SendTextMessage(recipientJID, job.Content)
		case "image", "document", "audio", "video":
			req := &whatsapp.MediaMessageRequest{
				To:       recipientJID,
				MediaURL: job.MediaURL,
				Type:     job.MessageType,
				Caption:  job.Content,
			}
			_, err = m.waClient.SendMediaMessage(req)
		default:
			err = fmt.Errorf("unsupported message type: %s", job.MessageType)
		}

		if err != nil {
			logrus.Errorf("Failed to send message to %s: %v", recipientJID, err)
			job.FailedCount++
		} else {
			logrus.Debugf("Message sent to %s", recipientJID)
			job.SentCount++
			sentInWindow++
		}

		// Update progress in database every 10 messages
		if (i+1)%10 == 0 || i == len(job.Recipients)-1 {
			m.db.Model(&database.BroadcastMessage{}).Where("id = ?", job.ID).Updates(map[string]interface{}{
				"sent_count":   job.SentCount,
				"failed_count": job.FailedCount,
			})
		}

		// Delay between messages
		if i < len(job.Recipients)-1 {
			time.Sleep(delayMs)
		}
	}
}

// GetBroadcastStatus returns the status of a broadcast
func (m *Manager) GetBroadcastStatus(broadcastID uint) (*BroadcastStatus, error) {
	var broadcastMsg database.BroadcastMessage
	if err := m.db.First(&broadcastMsg, broadcastID).Error; err != nil {
		return nil, err
	}

	progress := float64(0)
	if broadcastMsg.TotalRecipients > 0 {
		progress = float64(broadcastMsg.SentCount+broadcastMsg.FailedCount) / float64(broadcastMsg.TotalRecipients) * 100
	}

	return &BroadcastStatus{
		ID:              broadcastMsg.ID,
		BroadcastListID: broadcastMsg.BroadcastListID,
		Status:          broadcastMsg.Status,
		SentCount:       broadcastMsg.SentCount,
		FailedCount:     broadcastMsg.FailedCount,
		TotalRecipients: broadcastMsg.TotalRecipients,
		Progress:        progress,
		StartedAt:       broadcastMsg.StartedAt,
		CompletedAt:     broadcastMsg.CompletedAt,
		CreatedAt:       broadcastMsg.CreatedAt,
	}, nil
}

// CancelBroadcast cancels an active broadcast
func (m *Manager) CancelBroadcast(broadcastID uint) error {
	m.mu.RLock()
	job, exists := m.active[broadcastID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("broadcast not found or not active")
	}

	// Send cancel signal
	select {
	case job.cancel <- true:
		logrus.Infof("Cancel signal sent to broadcast %d", broadcastID)
	default:
		// Channel full or closed
	}

	// Update status in database
	m.db.Model(&database.BroadcastMessage{}).Where("id = ?", broadcastID).Update("status", "cancelled")

	return nil
}

// ListActiveBroadcasts returns all active broadcasts
func (m *Manager) ListActiveBroadcasts() []*BroadcastStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*BroadcastStatus, 0, len(m.active))
	for _, job := range m.active {
		progress := float64(0)
		if job.TotalRecipients > 0 {
			progress = float64(job.SentCount+job.FailedCount) / float64(job.TotalRecipients) * 100
		}

		status := &BroadcastStatus{
			ID:              job.ID,
			BroadcastListID: job.BroadcastListID,
			Status:          job.Status,
			SentCount:       job.SentCount,
			FailedCount:     job.FailedCount,
			TotalRecipients: job.TotalRecipients,
			Progress:        progress,
			StartedAt:       job.StartedAt,
			CompletedAt:     job.CompletedAt,
		}
		result = append(result, status)
	}

	return result
}