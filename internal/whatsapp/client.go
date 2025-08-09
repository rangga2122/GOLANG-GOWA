package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gowa-broadcast/internal/config"
	"gowa-broadcast/internal/database"

	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"gorm.io/gorm"
)

type Client struct {
	cfg      *config.Config
	db       *gorm.DB
	client   *whatsmeow.Client
	store    *sqlstore.Container
	device   *store.Device
	logger   waLog.Logger
	qrChan   chan string
	isReady  bool
}

type QRResponse struct {
	QRCode    string `json:"qr_code"`
	Timeout   int    `json:"timeout"`
	Connected bool   `json:"connected"`
}

func NewClient(cfg *config.Config, db *gorm.DB) (*Client, error) {
	// Create storages directory
	storageDir := "storages"
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %v", err)
	}

	// Initialize store
	dbLog := waLog.Stdout("Database", "INFO", true)
	container, err := sqlstore.New("sqlite3", filepath.Join(storageDir, "whatsapp_session.db"), dbLog)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %v", err)
	}

	// Get device store
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %v", err)
	}

	// Create logger
	clientLog := waLog.Stdout("Client", "INFO", true)

	// Create WhatsApp client
	client := whatsmeow.NewClient(deviceStore, clientLog)

	return &Client{
		cfg:    cfg,
		db:     db,
		client: client,
		store:  container,
		device: deviceStore,
		logger: clientLog,
		qrChan: make(chan string, 1),
	}, nil
}

func (c *Client) Start() error {
	// Add event handlers
	c.client.AddEventHandler(c.handleEvents)

	// Connect to WhatsApp
	if c.client.Store.ID == nil {
		// Not logged in, need QR code
		logrus.Info("Device not logged in, waiting for QR code scan...")
		return c.connectWithQR()
	} else {
		// Already logged in, try to connect
		logrus.Info("Device already logged in, connecting...")
		return c.client.Connect()
	}
}

func (c *Client) connectWithQR() error {
	qrChan, err := c.client.GetQRChannel(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get QR channel: %v", err)
	}

	go func() {
		for evt := range qrChan {
			if evt.Event == "code" {
				logrus.Info("QR code received")
				c.qrChan <- evt.Code
				
				// Save QR code to database
				device := &database.Device{
					JID:       "pending",
					Name:      c.cfg.App.OS,
					Platform:  "web",
					Connected: false,
					QRCode:    evt.Code,
					LastSeen:  time.Now(),
				}
				c.db.Create(device)
			} else {
				logrus.Infof("QR channel event: %s", evt.Event)
				if evt.Event == "success" {
					c.isReady = true
					logrus.Info("Successfully connected to WhatsApp")
					
					// Update device in database
					if c.client.Store.ID != nil {
						device := &database.Device{
							JID:       c.client.Store.ID.String(),
							Name:      c.cfg.App.OS,
							Platform:  "web",
							Connected: true,
							LastSeen:  time.Now(),
						}
						c.db.Where("jid = ?", "pending").Delete(&database.Device{})
						c.db.Create(device)
					}
				}
			}
		}
	}()

	return c.client.Connect()
}

func (c *Client) handleEvents(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		c.handleMessage(v)
	case *events.Receipt:
		c.handleReceipt(v)
	case *events.Connected:
		logrus.Info("Connected to WhatsApp")
		c.isReady = true
		
		// Update device status
		if c.client.Store.ID != nil {
			c.db.Model(&database.Device{}).Where("jid = ?", c.client.Store.ID.String()).Update("connected", true)
		}
	case *events.Disconnected:
		logrus.Warn("Disconnected from WhatsApp")
		c.isReady = false
		
		// Update device status
		if c.client.Store.ID != nil {
			c.db.Model(&database.Device{}).Where("jid = ?", c.client.Store.ID.String()).Update("connected", false)
		}
	case *events.LoggedOut:
		logrus.Warn("Logged out from WhatsApp")
		c.isReady = false
		
		// Remove device from database
		if c.client.Store.ID != nil {
			c.db.Where("jid = ?", c.client.Store.ID.String()).Delete(&database.Device{})
		}
	}
}

func (c *Client) handleMessage(evt *events.Message) {
	if evt.Info.IsFromMe {
		return // Skip own messages
	}

	// Save message to database if chat storage is enabled
	if c.cfg.WhatsApp.ChatStorage {
		msg := &database.Message{
			MessageID: evt.Info.ID,
			FromJID:   evt.Info.Sender.String(),
			ToJID:     evt.Info.Chat.String(),
			Type:      "text",
			Content:   evt.Message.GetConversation(),
			Timestamp: evt.Info.Timestamp,
			IsFromMe:  evt.Info.IsFromMe,
			IsRead:    false,
		}
		c.db.Create(msg)
	}

	// Auto mark as read if enabled
	if c.cfg.WhatsApp.AutoMarkRead {
		c.client.MarkRead([]types.MessageID{evt.Info.ID}, evt.Info.Timestamp, evt.Info.Chat, evt.Info.Sender)
	}

	// Auto reply if configured
	if c.cfg.WhatsApp.AutoReply != "" {
		c.SendTextMessage(evt.Info.Chat.String(), c.cfg.WhatsApp.AutoReply)
	}

	// Send webhook if configured
	if c.cfg.WhatsApp.Webhook != "" {
		go c.sendWebhook(evt)
	}
}

func (c *Client) handleReceipt(evt *events.Receipt) {
	// Update message read status
	if evt.Type == events.ReceiptTypeRead {
		for _, msgID := range evt.MessageIDs {
			c.db.Model(&database.Message{}).Where("message_id = ?", msgID).Update("is_read", true)
		}
	}
}

func (c *Client) sendWebhook(evt *events.Message) {
	// Create webhook payload
	payload := map[string]interface{}{
		"event": "message",
		"data": map[string]interface{}{
			"message_id": evt.Info.ID,
			"from":       evt.Info.Sender.String(),
			"to":         evt.Info.Chat.String(),
			"content":    evt.Message.GetConversation(),
			"timestamp":  evt.Info.Timestamp.Unix(),
			"is_group":   evt.Info.IsGroup,
		},
	}

	payloadBytes, _ := json.Marshal(payload)
	
	// Send to all configured webhooks
	webhooks := c.cfg.WhatsApp.ParseWebhooks()
	for _, webhookURL := range webhooks {
		go func(url string) {
			// TODO: Implement webhook delivery with retry logic
			logrus.Debugf("Sending webhook to %s: %s", url, string(payloadBytes))
		}(webhookURL)
	}
}

// GetQRCode returns the current QR code for login
func (c *Client) GetQRCode() (string, error) {
	if c.client.Store.ID != nil {
		return "", fmt.Errorf("already logged in")
	}

	select {
	case qr := <-c.qrChan:
		return qr, nil
	case <-time.After(30 * time.Second):
		return "", fmt.Errorf("timeout waiting for QR code")
	}
}

// IsReady returns true if the client is connected and ready
func (c *Client) IsReady() bool {
	return c.isReady && c.client.IsConnected()
}

// GetClient returns the underlying whatsmeow client
func (c *Client) GetClient() *whatsmeow.Client {
	return c.client
}

// Disconnect disconnects the client
func (c *Client) Disconnect() {
	c.client.Disconnect()
	c.isReady = false
}

// Logout logs out the client
func (c *Client) Logout() error {
	err := c.client.Logout()
	c.isReady = false
	return err
}