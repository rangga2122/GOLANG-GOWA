package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Initialize database connection
func Initialize(dbURI string) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	// Determine database type from URI
	if strings.HasPrefix(dbURI, "postgres://") || strings.HasPrefix(dbURI, "postgresql://") {
		// PostgreSQL
		db, err = gorm.Open(postgres.Open(dbURI), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
	} else {
		// SQLite (default)
		// Extract file path from URI
		filePath := strings.TrimPrefix(dbURI, "file:")
		if idx := strings.Index(filePath, "?"); idx != -1 {
			filePath = filePath[:idx]
		}

		// Create directory if it doesn't exist
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %v", dir, err)
		}

		db, err = gorm.Open(sqlite.Open(dbURI), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// Auto migrate tables
	if err := autoMigrate(db); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %v", err)
	}

	return db, nil
}

// Auto migrate all models
func autoMigrate(db *gorm.DB) error {
	err := db.AutoMigrate(
		&User{},
		&Device{},
		&Contact{},
		&Group{},
		&Message{},
		&BroadcastList{},
		&BroadcastRecipient{},
		&BroadcastMessage{},
		&ScheduledMessage{},
		&Webhook{},
		&WebhookLog{},
	)
	if err != nil {
		return err
	}

	// Create default admin user if not exists
	var adminCount int64
	db.Model(&User{}).Where("role = ?", "admin").Count(&adminCount)
	if adminCount == 0 {
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		adminUser := User{
			Username: "admin",
			Email:    "admin@gowa.local",
			Password: string(hashedPassword),
			FullName: "System Administrator",
			Role:     "admin",
			Active:   true,
		}
		db.Create(&adminUser)
		log.Println("Default admin user created: admin/admin123")
	}

	return nil
}

// User represents application users
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"uniqueIndex;not null" json:"username"`
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"not null" json:"-"` // Hidden from JSON
	FullName  string    `json:"full_name"`
	Role      string    `gorm:"default:'user'" json:"role"` // admin, user
	Active    bool      `gorm:"default:true" json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	Devices         []Device         `gorm:"foreignKey:UserID" json:"devices,omitempty"`
	Contacts        []Contact        `gorm:"foreignKey:UserID" json:"contacts,omitempty"`
	Groups          []Group          `gorm:"foreignKey:UserID" json:"groups,omitempty"`
	Messages        []Message        `gorm:"foreignKey:UserID" json:"messages,omitempty"`
	BroadcastLists  []BroadcastList  `gorm:"foreignKey:UserID" json:"broadcast_lists,omitempty"`
	Broadcasts      []BroadcastMessage `gorm:"foreignKey:UserID" json:"broadcasts,omitempty"`
	ScheduledMessages []ScheduledMessage `gorm:"foreignKey:UserID" json:"scheduled_messages,omitempty"`
}

// Device represents WhatsApp device information
type Device struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	JID         string    `gorm:"index" json:"jid"`
	Name        string    `json:"name"`
	Platform    string    `json:"platform"`
	Connected   bool      `json:"connected"`
	LastSeen    time.Time `json:"last_seen"`
	QRCode      string    `json:"qr_code,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relations
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// Contact represents WhatsApp contact
type Contact struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	JID         string    `gorm:"index" json:"jid"`
	Name        string    `json:"name"`
	PushName    string    `json:"push_name"`
	PhoneNumber string    `json:"phone_number"`
	IsGroup     bool      `json:"is_group"`
	IsBlocked   bool      `json:"is_blocked"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relations
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// Group represents WhatsApp group
type Group struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	JID         string    `gorm:"index" json:"jid"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	OwnerJID    string    `json:"owner_jid"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relations
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// Message represents WhatsApp message
type Message struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	MessageID   string    `gorm:"index" json:"message_id"`
	FromJID     string    `json:"from_jid"`
	ToJID       string    `json:"to_jid"`
	Type        string    `json:"type"`
	Content     string    `json:"content"`
	MediaURL    string    `json:"media_url,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	IsFromMe    bool      `json:"is_from_me"`
	IsRead      bool      `json:"is_read"`
	CreatedAt   time.Time `json:"created_at"`

	// Relations
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// BroadcastList represents a broadcast list
type BroadcastList struct {
	ID          uint                 `gorm:"primaryKey" json:"id"`
	UserID      uint                 `gorm:"not null;index" json:"user_id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	CreatedBy   string               `json:"created_by"`
	IsActive    bool                 `json:"is_active"`
	Recipients  []BroadcastRecipient `gorm:"foreignKey:BroadcastListID" json:"recipients"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`

	// Relations
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// BroadcastRecipient represents a recipient in a broadcast list
type BroadcastRecipient struct {
	ID              uint   `gorm:"primaryKey" json:"id"`
	BroadcastListID uint   `json:"broadcast_list_id"`
	JID             string `json:"jid"`
	Name            string `json:"name"`
	PhoneNumber     string `json:"phone_number"`
	IsActive        bool   `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
}

// BroadcastMessage represents a broadcast message
type BroadcastMessage struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	UserID          uint      `gorm:"not null;index" json:"user_id"`
	BroadcastListID uint      `json:"broadcast_list_id"`
	MessageType     string    `json:"message_type"`
	Content         string    `json:"content"`
	MediaURL        string    `json:"media_url,omitempty"`
	Status          string    `json:"status"` // pending, sending, completed, failed
	SentCount       int       `json:"sent_count"`
	FailedCount     int       `json:"failed_count"`
	TotalRecipients int       `json:"total_recipients"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	// Relations
	User          User          `gorm:"foreignKey:UserID" json:"user,omitempty"`
	BroadcastList BroadcastList `gorm:"foreignKey:BroadcastListID" json:"broadcast_list,omitempty"`
}

// ScheduledMessage represents a scheduled message
type ScheduledMessage struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	Name        string    `json:"name"`
	Recipients  string    `json:"recipients"` // JSON array of JIDs
	MessageType string    `json:"message_type"`
	Content     string    `json:"content"`
	MediaURL    string    `json:"media_url,omitempty"`
	ScheduledAt time.Time `json:"scheduled_at"`
	Status      string    `json:"status"` // pending, sent, failed, cancelled
	CronExpr    string    `json:"cron_expr,omitempty"` // For recurring messages
	IsRecurring bool      `json:"is_recurring"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relations
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// Webhook represents webhook configuration
type Webhook struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	URL       string    `gorm:"not null" json:"url"`
	Secret    string    `json:"secret"`
	Events    string    `gorm:"type:text" json:"events"` // JSON array of events
	Headers   string    `gorm:"type:text" json:"headers"` // JSON object of headers
	Active    bool      `gorm:"default:true" json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WebhookLog represents webhook delivery log
type WebhookLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	WebhookID    uint      `json:"webhook_id"`
	Event        string    `json:"event"`
	Payload      string    `gorm:"type:text" json:"payload"`
	StatusCode   int       `json:"status_code"`
	ResponseBody string    `gorm:"type:text" json:"response_body"`
	Error        string    `json:"error"`
	CreatedAt    time.Time `json:"created_at"`
}