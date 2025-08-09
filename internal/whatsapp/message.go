package whatsapp

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
	waProto "go.mau.fi/whatsmeow/binary/proto"
)

type MessageRequest struct {
	To      string `json:"to" binding:"required"`
	Message string `json:"message" binding:"required"`
	Type    string `json:"type,omitempty"` // text, image, document, audio, video
}

type MediaMessageRequest struct {
	To       string `json:"to" binding:"required"`
	Message  string `json:"message,omitempty"`
	MediaURL string `json:"media_url" binding:"required"`
	Type     string `json:"type" binding:"required"` // image, document, audio, video
	FileName string `json:"file_name,omitempty"`
	Caption  string `json:"caption,omitempty"`
}

type LocationMessageRequest struct {
	To        string  `json:"to" binding:"required"`
	Latitude  float64 `json:"latitude" binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
	Name      string  `json:"name,omitempty"`
	Address   string  `json:"address,omitempty"`
}

type ContactMessageRequest struct {
	To          string `json:"to" binding:"required"`
	DisplayName string `json:"display_name" binding:"required"`
	VCard       string `json:"vcard" binding:"required"`
}

type MessageResponse struct {
	Success   bool   `json:"success"`
	MessageID string `json:"message_id,omitempty"`
	Error     string `json:"error,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// SendTextMessage sends a text message
func (c *Client) SendTextMessage(to, message string) (*MessageResponse, error) {
	if !c.IsReady() {
		return &MessageResponse{
			Success:   false,
			Error:     "WhatsApp client not ready",
			Timestamp: time.Now().Unix(),
		}, fmt.Errorf("client not ready")
	}

	// Parse JID
	jid, err := c.parseJID(to)
	if err != nil {
		return &MessageResponse{
			Success:   false,
			Error:     fmt.Sprintf("Invalid JID: %v", err),
			Timestamp: time.Now().Unix(),
		}, err
	}

	// Create message
	msg := &waProto.Message{
		Conversation: proto.String(message),
	}

	// Send message
	resp, err := c.client.SendMessage(context.Background(), jid, msg)
	if err != nil {
		return &MessageResponse{
			Success:   false,
			Error:     fmt.Sprintf("Failed to send message: %v", err),
			Timestamp: time.Now().Unix(),
		}, err
	}

	return &MessageResponse{
		Success:   true,
		MessageID: resp.ID,
		Timestamp: resp.Timestamp.Unix(),
	}, nil
}

// SendMediaMessage sends a media message
func (c *Client) SendMediaMessage(req *MediaMessageRequest) (*MessageResponse, error) {
	if !c.IsReady() {
		return &MessageResponse{
			Success:   false,
			Error:     "WhatsApp client not ready",
			Timestamp: time.Now().Unix(),
		}, fmt.Errorf("client not ready")
	}

	// Parse JID
	jid, err := c.parseJID(req.To)
	if err != nil {
		return &MessageResponse{
			Success:   false,
			Error:     fmt.Sprintf("Invalid JID: %v", err),
			Timestamp: time.Now().Unix(),
		}, err
	}

	// Download media
	mediaData, err := c.downloadMedia(req.MediaURL)
	if err != nil {
		return &MessageResponse{
			Success:   false,
			Error:     fmt.Sprintf("Failed to download media: %v", err),
			Timestamp: time.Now().Unix(),
		}, err
	}

	// Upload media
	uploaded, err := c.client.Upload(context.Background(), mediaData, whatsmeow.MediaType(req.Type))
	if err != nil {
		return &MessageResponse{
			Success:   false,
			Error:     fmt.Sprintf("Failed to upload media: %v", err),
			Timestamp: time.Now().Unix(),
		}, err
	}

	// Create message based on type
	var msg *waProto.Message
	switch strings.ToLower(req.Type) {
	case "image":
		msg = &waProto.Message{
			ImageMessage: &waProto.ImageMessage{
				Url:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				FileEncSha256: uploaded.FileEncSHA256,
				FileSha256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaData))),
				Caption:       proto.String(req.Caption),
			},
		}
	case "document":
		fileName := req.FileName
		if fileName == "" {
			fileName = "document"
		}
		mimeType := mime.TypeByExtension(filepath.Ext(fileName))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		msg = &waProto.Message{
			DocumentMessage: &waProto.DocumentMessage{
				Url:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				FileEncSha256: uploaded.FileEncSHA256,
				FileSha256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaData))),
				FileName:      proto.String(fileName),
				Mimetype:      proto.String(mimeType),
				Caption:       proto.String(req.Caption),
			},
		}
	case "audio":
		msg = &waProto.Message{
			AudioMessage: &waProto.AudioMessage{
				Url:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				FileEncSha256: uploaded.FileEncSHA256,
				FileSha256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaData))),
				Mimetype:      proto.String("audio/ogg; codecs=opus"),
			},
		}
	case "video":
		msg = &waProto.Message{
			VideoMessage: &waProto.VideoMessage{
				Url:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				FileEncSha256: uploaded.FileEncSHA256,
				FileSha256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaData))),
				Caption:       proto.String(req.Caption),
			},
		}
	default:
		return &MessageResponse{
			Success:   false,
			Error:     "Unsupported media type",
			Timestamp: time.Now().Unix(),
		}, fmt.Errorf("unsupported media type: %s", req.Type)
	}

	// Send message
	resp, err := c.client.SendMessage(context.Background(), jid, msg)
	if err != nil {
		return &MessageResponse{
			Success:   false,
			Error:     fmt.Sprintf("Failed to send message: %v", err),
			Timestamp: time.Now().Unix(),
		}, err
	}

	return &MessageResponse{
		Success:   true,
		MessageID: resp.ID,
		Timestamp: resp.Timestamp.Unix(),
	}, nil
}

// SendLocationMessage sends a location message
func (c *Client) SendLocationMessage(req *LocationMessageRequest) (*MessageResponse, error) {
	if !c.IsReady() {
		return &MessageResponse{
			Success:   false,
			Error:     "WhatsApp client not ready",
			Timestamp: time.Now().Unix(),
		}, fmt.Errorf("client not ready")
	}

	// Parse JID
	jid, err := c.parseJID(req.To)
	if err != nil {
		return &MessageResponse{
			Success:   false,
			Error:     fmt.Sprintf("Invalid JID: %v", err),
			Timestamp: time.Now().Unix(),
		}, err
	}

	// Create location message
	msg := &waProto.Message{
		LocationMessage: &waProto.LocationMessage{
			DegreesLatitude:  proto.Float64(req.Latitude),
			DegreesLongitude: proto.Float64(req.Longitude),
			Name:             proto.String(req.Name),
			Address:          proto.String(req.Address),
		},
	}

	// Send message
	resp, err := c.client.SendMessage(context.Background(), jid, msg)
	if err != nil {
		return &MessageResponse{
			Success:   false,
			Error:     fmt.Sprintf("Failed to send message: %v", err),
			Timestamp: time.Now().Unix(),
		}, err
	}

	return &MessageResponse{
		Success:   true,
		MessageID: resp.ID,
		Timestamp: resp.Timestamp.Unix(),
	}, nil
}

// SendContactMessage sends a contact message
func (c *Client) SendContactMessage(req *ContactMessageRequest) (*MessageResponse, error) {
	if !c.IsReady() {
		return &MessageResponse{
			Success:   false,
			Error:     "WhatsApp client not ready",
			Timestamp: time.Now().Unix(),
		}, fmt.Errorf("client not ready")
	}

	// Parse JID
	jid, err := c.parseJID(req.To)
	if err != nil {
		return &MessageResponse{
			Success:   false,
			Error:     fmt.Sprintf("Invalid JID: %v", err),
			Timestamp: time.Now().Unix(),
		}, err
	}

	// Create contact message
	msg := &waProto.Message{
		ContactMessage: &waProto.ContactMessage{
			DisplayName: proto.String(req.DisplayName),
			Vcard:       proto.String(req.VCard),
		},
	}

	// Send message
	resp, err := c.client.SendMessage(context.Background(), jid, msg)
	if err != nil {
		return &MessageResponse{
			Success:   false,
			Error:     fmt.Sprintf("Failed to send message: %v", err),
			Timestamp: time.Now().Unix(),
		}, err
	}

	return &MessageResponse{
		Success:   true,
		MessageID: resp.ID,
		Timestamp: resp.Timestamp.Unix(),
	}, nil
}

// parseJID parses a phone number or JID string into a types.JID
func (c *Client) parseJID(to string) (types.JID, error) {
	if strings.Contains(to, "@") {
		// Already a JID
		return types.ParseJID(to)
	}

	// Phone number, convert to JID
	phoneNumber := strings.ReplaceAll(to, "+", "")
	phoneNumber = strings.ReplaceAll(phoneNumber, " ", "")
	phoneNumber = strings.ReplaceAll(phoneNumber, "-", "")

	if !strings.HasSuffix(phoneNumber, "@s.whatsapp.net") {
		phoneNumber += "@s.whatsapp.net"
	}

	return types.ParseJID(phoneNumber)
}

// downloadMedia downloads media from URL
func (c *Client) downloadMedia(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download media: status %d", resp.StatusCode)
	}

	// Read response body
	data := make([]byte, resp.ContentLength)
	_, err = resp.Body.Read(data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// GenerateMessageID generates a unique message ID
func (c *Client) GenerateMessageID() string {
	return uuid.New().String()
}