package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"gowa-broadcast/internal/config"
	"gowa-broadcast/internal/database"
	"gowa-broadcast/internal/server"
	"gowa-broadcast/internal/whatsapp"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		logrus.Warn("No .env file found, using environment variables")
	}

	// Parse command line flags
	var (
		port = flag.String("port", "", "Server port")
		debug = flag.Bool("debug", false, "Enable debug mode")
		osName = flag.String("os", "", "OS name for WhatsApp")
		basicAuth = flag.String("basic-auth", "", "Basic auth credentials (user:pass,user2:pass2)")
		basePath = flag.String("base-path", "", "Base path for subpath deployment")
		autoReply = flag.String("autoreply", "", "Auto reply message")
		autoMarkRead = flag.Bool("auto-mark-read", false, "Auto mark read incoming messages")
		webhook = flag.String("webhook", "", "Webhook URL for received messages")
		webhookSecret = flag.String("webhook-secret", "", "Webhook secret for validation")
		dbURI = flag.String("db-uri", "", "Database connection URI")
	)
	flag.Parse()

	// Load configuration
	cfg := config.Load()

	// Override config with command line flags if provided
	if *port != "" {
		cfg.App.Port = *port
	}
	if *debug {
		cfg.App.Debug = *debug
	}
	if *osName != "" {
		cfg.App.OS = *osName
	}
	if *basicAuth != "" {
		cfg.App.BasicAuth = *basicAuth
	}
	if *basePath != "" {
		cfg.App.BasePath = *basePath
	}
	if *autoReply != "" {
		cfg.WhatsApp.AutoReply = *autoReply
	}
	if *autoMarkRead {
		cfg.WhatsApp.AutoMarkRead = *autoMarkRead
	}
	if *webhook != "" {
		cfg.WhatsApp.Webhook = *webhook
	}
	if *webhookSecret != "" {
		cfg.WhatsApp.WebhookSecret = *webhookSecret
	}
	if *dbURI != "" {
		cfg.Database.URI = *dbURI
	}

	// Setup logging
	if cfg.App.Debug {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	} else {
		logrus.SetLevel(logrus.InfoLevel)
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}

	// Check command
	args := flag.Args()
	if len(args) == 0 {
		fmt.Println("Usage: gowa-broadcast [rest|help]")
		fmt.Println("Commands:")
		fmt.Println("  rest    Start REST API server")
		fmt.Println("  help    Show this help message")
		os.Exit(1)
	}

	command := strings.ToLower(args[0])
	switch command {
	case "rest":
		startRESTServer(cfg)
	case "help":
		showHelp()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		showHelp()
		os.Exit(1)
	}
}

func startRESTServer(cfg *config.Config) {
	logrus.Info("Starting GOWA Broadcast REST API Server...")

	// Initialize database
	db, err := database.Initialize(cfg.Database.URI)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// Initialize WhatsApp client
	waClient, err := whatsapp.NewClient(cfg, db)
	if err != nil {
		log.Fatal("Failed to initialize WhatsApp client:", err)
	}

	// Start WhatsApp client
	if err := waClient.Start(); err != nil {
		log.Fatal("Failed to start WhatsApp client:", err)
	}

	// Initialize and start HTTP server
	server := server.NewServer(cfg, db, waClient)
	if err := server.Start(); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

func showHelp() {
	fmt.Println("GOWA Broadcast - WhatsApp REST API with Broadcast Features")
	fmt.Println("")
	fmt.Println("Usage: gowa-broadcast [command] [options]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  rest    Start REST API server")
	fmt.Println("  help    Show this help message")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --port string              Server port (default: 3000)")
	fmt.Println("  --debug                    Enable debug mode")
	fmt.Println("  --os string                OS name for WhatsApp (default: GOWA-Broadcast)")
	fmt.Println("  --basic-auth string        Basic auth credentials (user:pass,user2:pass2)")
	fmt.Println("  --base-path string         Base path for subpath deployment")
	fmt.Println("  --autoreply string         Auto reply message")
	fmt.Println("  --auto-mark-read           Auto mark read incoming messages")
	fmt.Println("  --webhook string           Webhook URL for received messages")
	fmt.Println("  --webhook-secret string    Webhook secret for validation")
	fmt.Println("  --db-uri string            Database connection URI")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  gowa-broadcast rest --port 8080 --debug")
	fmt.Println("  gowa-broadcast rest --basic-auth admin:secret --webhook http://localhost:8080/webhook")
	fmt.Println("")
	fmt.Println("Environment Variables:")
	fmt.Println("  APP_PORT, APP_DEBUG, APP_OS, APP_BASIC_AUTH, APP_BASE_PATH")
	fmt.Println("  DB_URI, WHATSAPP_AUTO_REPLY, WHATSAPP_AUTO_MARK_READ")
	fmt.Println("  WHATSAPP_WEBHOOK, WHATSAPP_WEBHOOK_SECRET")
	fmt.Println("  BROADCAST_RATE_LIMIT, BROADCAST_DELAY_MS, BROADCAST_MAX_RECIPIENTS")
}