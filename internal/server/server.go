package server

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gowa-broadcast/internal/auth"
	"gowa-broadcast/internal/broadcast"
	"gowa-broadcast/internal/config"
	"gowa-broadcast/internal/database"
	"gowa-broadcast/internal/middleware"
	"gowa-broadcast/internal/whatsapp"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type Server struct {
	cfg             *config.Config
	db              *gorm.DB
	waClient        *whatsapp.Client
	broadcastMgr    *broadcast.Manager
	authService     *auth.AuthService
	authHandlers    *AuthHandlers
	router          *gin.Engine
	basicAuthUsers  map[string]string
}

func NewServer(cfg *config.Config, db *gorm.DB, waClient *whatsapp.Client) *Server {
	// Setup Gin mode
	if !cfg.App.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create broadcast manager
	broadcastMgr := broadcast.NewManager(cfg, db, waClient)

	// Create auth service
	authService := auth.NewAuthService(db, cfg.JWT.Secret)

	// Parse basic auth users
	basicAuthUsers := cfg.App.ParseBasicAuth()

	server := &Server{
		cfg:            cfg,
		db:             db,
		waClient:       waClient,
		broadcastMgr:   broadcastMgr,
		authService:    authService,
		basicAuthUsers: basicAuthUsers,
	}

	// Create auth handlers
	server.authHandlers = NewAuthHandlers(authService)

	server.setupRoutes()
	return server
}

func (s *Server) setupRoutes() {
	s.router = gin.New()

	// Middleware
	s.router.Use(gin.Logger())
	s.router.Use(gin.Recovery())
	s.router.Use(s.corsMiddleware())

	// Base path
	var api *gin.RouterGroup
	if s.cfg.App.BasePath != "" {
		api = s.router.Group(s.cfg.App.BasePath)
	} else {
		api = s.router.Group("")
	}

	// Public routes
	api.GET("/", s.handleIndex)
	api.GET("/health", s.handleHealth)

	// Authentication routes (public)
	auth := api.Group("/auth")
	{
		auth.POST("/login", s.authHandlers.Login)
		auth.POST("/validate", middleware.AuthMiddleware(s.authService), s.authHandlers.ValidateToken)
	}

	// Protected routes with JWT authentication
	protected := api.Group("/")
	protected.Use(middleware.AuthMiddleware(s.authService))

	// User management routes (for authenticated users)
	users := protected.Group("/users")
	{
		// Profile routes (accessible by all authenticated users)
		users.GET("/profile", s.authHandlers.GetProfile)
		users.PUT("/profile", s.authHandlers.UpdateProfile)
		users.POST("/change-password", s.authHandlers.ChangeMyPassword)

		// Admin only routes
		adminUsers := users.Group("/")
		adminUsers.Use(middleware.AdminOnlyMiddleware())
		{
			adminUsers.POST("/", s.authHandlers.CreateUser)
			adminUsers.GET("/", s.authHandlers.GetUsers)
			adminUsers.GET("/:id", s.authHandlers.GetUser)
			adminUsers.PUT("/:id", s.authHandlers.UpdateUser)
			adminUsers.DELETE("/:id", s.authHandlers.DeleteUser)
			adminUsers.POST("/:id/change-password", s.authHandlers.ChangePassword)
		}
	}

	// Legacy protected routes with basic auth (for backward compatibility)
	legacy := api.Group("/legacy")
	if len(s.basicAuthUsers) > 0 {
		legacy.Use(s.basicAuthMiddleware())
	}

	// WhatsApp routes
	wa := protected.Group("/whatsapp")
	{
		wa.GET("/qr", s.handleGetQR)
		wa.GET("/status", s.handleGetStatus)
		wa.POST("/logout", s.handleLogout)
		wa.GET("/contacts", s.handleGetContacts)
		wa.GET("/groups", s.handleGetGroups)
	}

	// Message routes
	messages := protected.Group("/messages")
	{
		messages.POST("/text", s.handleSendText)
		messages.POST("/media", s.handleSendMedia)
		messages.POST("/location", s.handleSendLocation)
		messages.POST("/contact", s.handleSendContact)
		messages.GET("/", s.handleGetMessages)
	}

	// Broadcast List routes
	broadcastLists := protected.Group("/broadcast-lists")
	{
		broadcastLists.GET("/", s.handleGetBroadcastLists)
		broadcastLists.POST("/", s.handleCreateBroadcastList)
		broadcastLists.GET("/:id", s.handleGetBroadcastList)
		broadcastLists.PUT("/:id", s.handleUpdateBroadcastList)
		broadcastLists.DELETE("/:id", s.handleDeleteBroadcastList)
		broadcastLists.POST("/:id/recipients", s.handleAddRecipients)
		broadcastLists.DELETE("/:id/recipients/:recipientId", s.handleRemoveRecipient)
	}

	// Broadcast routes
	broadcasts := protected.Group("/broadcasts")
	{
		broadcasts.POST("/", s.handleCreateBroadcast)
		broadcasts.GET("/:id/status", s.handleGetBroadcastStatus)
		broadcasts.POST("/:id/cancel", s.handleCancelBroadcast)
		broadcasts.GET("/active", s.handleGetActiveBroadcasts)
		broadcasts.GET("/history", s.handleGetBroadcastHistory)
	}

	// Scheduled messages routes
	scheduled := protected.Group("/scheduled")
	{
		scheduled.GET("/", s.handleGetScheduledMessages)
		scheduled.POST("/", s.handleCreateScheduledMessage)
		scheduled.GET("/:id", s.handleGetScheduledMessage)
		scheduled.PUT("/:id", s.handleUpdateScheduledMessage)
		scheduled.DELETE("/:id", s.handleDeleteScheduledMessage)
	}

	// Statistics routes
	stats := protected.Group("/stats")
	{
		stats.GET("/dashboard", s.handleGetDashboardStats)
		stats.GET("/messages", s.handleGetMessageStats)
		stats.GET("/broadcasts", s.handleGetBroadcastStats)
	}

	// Webhook routes
	webhooks := protected.Group("/webhooks")
	{
		webhooks.POST("/", s.handleCreateWebhook)
		webhooks.GET("/", s.handleGetWebhooks)
		webhooks.GET("/:id", s.handleGetWebhook)
		webhooks.PUT("/:id", s.handleUpdateWebhook)
		webhooks.DELETE("/:id", s.handleDeleteWebhook)
		webhooks.POST("/:id/toggle", s.handleToggleWebhook)
		webhooks.GET("/:id/logs", s.handleGetWebhookLogs)
	}
}

func (s *Server) Start() error {
	logrus.Infof("Starting HTTP server on port %s", s.cfg.App.Port)
	return s.router.Run(":" + s.cfg.App.Port)
}

// Middleware
func (s *Server) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func (s *Server) basicAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.Header("WWW-Authenticate", `Basic realm="Restricted"`)
			c.JSON(401, gin.H{"error": "Authorization required"})
			c.Abort()
			return
		}

		if !strings.HasPrefix(auth, "Basic ") {
			c.JSON(401, gin.H{"error": "Invalid authorization format"})
			c.Abort()
			return
		}

		payload, err := base64.StdEncoding.DecodeString(auth[6:])
		if err != nil {
			c.JSON(401, gin.H{"error": "Invalid authorization encoding"})
			c.Abort()
			return
		}

		parts := strings.SplitN(string(payload), ":", 2)
		if len(parts) != 2 {
			c.JSON(401, gin.H{"error": "Invalid authorization format"})
			c.Abort()
			return
		}

		username, password := parts[0], parts[1]
		if expectedPassword, exists := s.basicAuthUsers[username]; !exists || expectedPassword != password {
			c.Header("WWW-Authenticate", `Basic realm="Restricted"`)
			c.JSON(401, gin.H{"error": "Invalid credentials"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Handlers
func (s *Server) handleIndex(c *gin.Context) {
	c.JSON(200, gin.H{
		"name":        "GOWA Broadcast",
		"description": "WhatsApp REST API with Broadcast Features",
		"version":     "1.0.0",
		"status":      "running",
		"timestamp":   time.Now().Unix(),
	})
}

func (s *Server) handleHealth(c *gin.Context) {
	whatsappStatus := "disconnected"
	if s.waClient.IsReady() {
		whatsappStatus = "connected"
	}

	c.JSON(200, gin.H{
		"status":    "healthy",
		"whatsapp":  whatsappStatus,
		"timestamp": time.Now().Unix(),
	})
}

func (s *Server) handleGetQR(c *gin.Context) {
	if s.waClient.GetClient().Store.ID != nil {
		c.JSON(200, gin.H{
			"connected": true,
			"message":   "Already connected to WhatsApp",
		})
		return
	}

	qrCode, err := s.waClient.GetQRCode()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"qr_code":   qrCode,
		"connected": false,
		"timeout":   30,
	})
}

func (s *Server) handleGetStatus(c *gin.Context) {
	var device database.Device
	connected := s.waClient.IsReady()
	jid := ""

	if s.waClient.GetClient().Store.ID != nil {
		jid = s.waClient.GetClient().Store.ID.String()
		s.db.Where("jid = ?", jid).First(&device)
	}

	c.JSON(200, gin.H{
		"connected":  connected,
		"jid":        jid,
		"device":     device,
		"timestamp":  time.Now().Unix(),
	})
}

func (s *Server) handleLogout(c *gin.Context) {
	if err := s.waClient.Logout(); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Logged out successfully"})
}

func (s *Server) handleGetContacts(c *gin.Context) {
	// Get current user ID
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	var contacts []database.Contact
	query := s.db.Where("user_id = ? AND is_group = ?", userID, false)

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset := (page - 1) * limit

	// Search
	if search := c.Query("search"); search != "" {
		query = query.Where("name LIKE ? OR phone_number LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)
	query.Offset(offset).Limit(limit).Find(&contacts)

	c.JSON(200, gin.H{
		"contacts": contacts,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

func (s *Server) handleGetGroups(c *gin.Context) {
	// Get current user ID
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	var groups []database.Group
	query := s.db.Model(&database.Group{}).Where("user_id = ?", userID)

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset := (page - 1) * limit

	// Search
	if search := c.Query("search"); search != "" {
		query = query.Where("name LIKE ? OR description LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)
	query.Offset(offset).Limit(limit).Find(&groups)

	c.JSON(200, gin.H{
		"groups": groups,
		"total":  total,
		"page":   page,
		"limit":  limit,
	})
}

func (s *Server) handleSendText(c *gin.Context) {
	var req whatsapp.MessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	resp, err := s.waClient.SendTextMessage(req.To, req.Message)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, resp)
}

func (s *Server) handleSendMedia(c *gin.Context) {
	var req whatsapp.MediaMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	resp, err := s.waClient.SendMediaMessage(&req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, resp)
}

func (s *Server) handleSendLocation(c *gin.Context) {
	var req whatsapp.LocationMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	resp, err := s.waClient.SendLocationMessage(&req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, resp)
}

func (s *Server) handleSendContact(c *gin.Context) {
	var req whatsapp.ContactMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	resp, err := s.waClient.SendContactMessage(&req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, resp)
}

func (s *Server) handleGetMessages(c *gin.Context) {
	// Get current user ID
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	var messages []database.Message
	query := s.db.Model(&database.Message{}).Where("user_id = ?", userID)

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset := (page - 1) * limit

	// Filters
	if chat := c.Query("chat"); chat != "" {
		query = query.Where("to_jid = ? OR from_jid = ?", chat, chat)
	}
	if msgType := c.Query("type"); msgType != "" {
		query = query.Where("type = ?", msgType)
	}

	var total int64
	query.Count(&total)
	query.Order("timestamp DESC").Offset(offset).Limit(limit).Find(&messages)

	c.JSON(200, gin.H{
		"messages": messages,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}