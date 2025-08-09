package server

import (
	"encoding/json"
	"net/http"
	"time"

	"gowa-broadcast/internal/database"
	"gowa-broadcast/internal/middleware"

	"github.com/gin-gonic/gin"
)

type DashboardStats struct {
	TotalMessages       int64                    `json:"total_messages"`
	TotalBroadcasts     int64                    `json:"total_broadcasts"`
	TotalBroadcastLists int64                    `json:"total_broadcast_lists"`
	TotalContacts       int64                    `json:"total_contacts"`
	TotalGroups         int64                    `json:"total_groups"`
	ActiveBroadcasts    int                      `json:"active_broadcasts"`
	PendingScheduled    int64                    `json:"pending_scheduled"`
	WhatsAppStatus      string                   `json:"whatsapp_status"`
	RecentActivity      []RecentActivityItem     `json:"recent_activity"`
	MessageStats        MessageStatsResponse     `json:"message_stats"`
	BroadcastStats      BroadcastStatsResponse   `json:"broadcast_stats"`
}

type RecentActivityItem struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
}

type MessageStatsResponse struct {
	Today     MessageStatsPeriod `json:"today"`
	Yesterday MessageStatsPeriod `json:"yesterday"`
	ThisWeek  MessageStatsPeriod `json:"this_week"`
	ThisMonth MessageStatsPeriod `json:"this_month"`
	Daily     []DailyStats       `json:"daily"`
}

type MessageStatsPeriod struct {
	Total    int64 `json:"total"`
	Sent     int64 `json:"sent"`
	Received int64 `json:"received"`
}

type BroadcastStatsResponse struct {
	Today     BroadcastStatsPeriod `json:"today"`
	Yesterday BroadcastStatsPeriod `json:"yesterday"`
	ThisWeek  BroadcastStatsPeriod `json:"this_week"`
	ThisMonth BroadcastStatsPeriod `json:"this_month"`
	Daily     []DailyBroadcastStats `json:"daily"`
}

type BroadcastStatsPeriod struct {
	Total       int64 `json:"total"`
	Completed   int64 `json:"completed"`
	Failed      int64 `json:"failed"`
	Cancelled   int64 `json:"cancelled"`
	TotalSent   int64 `json:"total_sent"`
	TotalFailed int64 `json:"total_failed"`
}

type DailyStats struct {
	Date     string `json:"date"`
	Total    int64  `json:"total"`
	Sent     int64  `json:"sent"`
	Received int64  `json:"received"`
}

type DailyBroadcastStats struct {
	Date        string `json:"date"`
	Total       int64  `json:"total"`
	Completed   int64  `json:"completed"`
	Failed      int64  `json:"failed"`
	TotalSent   int64  `json:"total_sent"`
	TotalFailed int64  `json:"total_failed"`
}

func (s *Server) handleGetDashboardStats(c *gin.Context) {
	// Get current user ID
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}
	stats := &DashboardStats{}

	// Get total counts for current user
	s.db.Model(&database.Message{}).Where("user_id = ?", userID).Count(&stats.TotalMessages)
	s.db.Model(&database.BroadcastMessage{}).Where("user_id = ?", userID).Count(&stats.TotalBroadcasts)
	s.db.Model(&database.BroadcastList{}).Where("user_id = ?", userID).Count(&stats.TotalBroadcastLists)
	s.db.Model(&database.Contact{}).Where("user_id = ? AND is_group = ?", userID, false).Count(&stats.TotalContacts)
	s.db.Model(&database.Group{}).Where("user_id = ?", userID).Count(&stats.TotalGroups)
	s.db.Model(&database.ScheduledMessage{}).Where("user_id = ? AND status = ?", userID, "pending").Count(&stats.PendingScheduled)

	// Get active broadcasts count (filtered by user)
	stats.ActiveBroadcasts = len(s.broadcastMgr.ListActiveBroadcasts()) // TODO: Filter by user

	// WhatsApp status
	if s.waClient.IsReady() {
		stats.WhatsAppStatus = "connected"
	} else {
		stats.WhatsAppStatus = "disconnected"
	}

	// Get recent activity for current user
	stats.RecentActivity = s.getRecentActivity(userID)

	// Get message stats for current user
	stats.MessageStats = s.getMessageStats(userID)

	// Get broadcast stats for current user
	stats.BroadcastStats = s.getBroadcastStats(userID)

	c.JSON(200, stats)
}

func (s *Server) handleGetMessageStats(c *gin.Context) {
	// Get current user ID
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	stats := s.getMessageStats(userID)
	c.JSON(200, stats)
}

func (s *Server) handleGetBroadcastStats(c *gin.Context) {
	// Get current user ID
	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	stats := s.getBroadcastStats(userID)
	c.JSON(200, stats)
}

func (s *Server) getRecentActivity(userID uint) []RecentActivityItem {
	activity := make([]RecentActivityItem, 0)

	// Recent messages for current user
	var recentMessages []database.Message
	s.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(5).Find(&recentMessages)
	for _, msg := range recentMessages {
		activity = append(activity, RecentActivityItem{
			Type:        "message",
			Description: "Message " + msg.Type + " from " + msg.FromJID,
			Timestamp:   msg.CreatedAt,
		})
	}

	// Recent broadcasts for current user
	var recentBroadcasts []database.BroadcastMessage
	s.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(3).Find(&recentBroadcasts)
	for _, broadcast := range recentBroadcasts {
		activity = append(activity, RecentActivityItem{
			Type:        "broadcast",
			Description: "Broadcast " + broadcast.Status + " with " + string(rune(broadcast.TotalRecipients)) + " recipients",
			Timestamp:   broadcast.CreatedAt,
		})
	}

	// Sort by timestamp (most recent first)
	for i := 0; i < len(activity)-1; i++ {
		for j := i + 1; j < len(activity); j++ {
			if activity[i].Timestamp.Before(activity[j].Timestamp) {
				activity[i], activity[j] = activity[j], activity[i]
			}
		}
	}

	// Limit to 10 items
	if len(activity) > 10 {
		activity = activity[:10]
	}

	return activity
}

func (s *Server) getMessageStats(userID uint) MessageStatsResponse {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	weekStart := today.AddDate(0, 0, -int(today.Weekday()))
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	stats := MessageStatsResponse{}

	// Today
	stats.Today = s.getMessageStatsForPeriod(userID, today, today.AddDate(0, 0, 1))

	// Yesterday
	stats.Yesterday = s.getMessageStatsForPeriod(userID, yesterday, today)

	// This week
	stats.ThisWeek = s.getMessageStatsForPeriod(userID, weekStart, today.AddDate(0, 0, 1))

	// This month
	stats.ThisMonth = s.getMessageStatsForPeriod(userID, monthStart, today.AddDate(0, 0, 1))

	// Daily stats for last 7 days
	stats.Daily = s.getDailyMessageStats(userID, 7)

	return stats
}

func (s *Server) getMessageStatsForPeriod(userID uint, start, end time.Time) MessageStatsPeriod {
	var stats MessageStatsPeriod

	s.db.Model(&database.Message{}).Where("user_id = ? AND created_at >= ? AND created_at < ?", userID, start, end).Count(&stats.Total)
	s.db.Model(&database.Message{}).Where("user_id = ? AND created_at >= ? AND created_at < ? AND is_from_me = ?", userID, start, end, true).Count(&stats.Sent)
	s.db.Model(&database.Message{}).Where("user_id = ? AND created_at >= ? AND created_at < ? AND is_from_me = ?", userID, start, end, false).Count(&stats.Received)

	return stats
}

func (s *Server) getDailyMessageStats(userID uint, days int) []DailyStats {
	stats := make([]DailyStats, 0, days)
	now := time.Now()

	for i := days - 1; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
		dayEnd := dayStart.AddDate(0, 0, 1)

		dailyStat := DailyStats{
			Date: dayStart.Format("2006-01-02"),
		}

		s.db.Model(&database.Message{}).Where("user_id = ? AND created_at >= ? AND created_at < ?", userID, dayStart, dayEnd).Count(&dailyStat.Total)
		s.db.Model(&database.Message{}).Where("user_id = ? AND created_at >= ? AND created_at < ? AND is_from_me = ?", userID, dayStart, dayEnd, true).Count(&dailyStat.Sent)
		s.db.Model(&database.Message{}).Where("user_id = ? AND created_at >= ? AND created_at < ? AND is_from_me = ?", userID, dayStart, dayEnd, false).Count(&dailyStat.Received)

		stats = append(stats, dailyStat)
	}

	return stats
}

func (s *Server) getBroadcastStats(userID uint) BroadcastStatsResponse {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	weekStart := today.AddDate(0, 0, -int(today.Weekday()))
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	stats := BroadcastStatsResponse{}

	// Today
	stats.Today = s.getBroadcastStatsForPeriod(userID, today, today.AddDate(0, 0, 1))

	// Yesterday
	stats.Yesterday = s.getBroadcastStatsForPeriod(userID, yesterday, today)

	// This week
	stats.ThisWeek = s.getBroadcastStatsForPeriod(userID, weekStart, today.AddDate(0, 0, 1))

	// This month
	stats.ThisMonth = s.getBroadcastStatsForPeriod(userID, monthStart, today.AddDate(0, 0, 1))

	// Daily stats for last 7 days
	stats.Daily = s.getDailyBroadcastStats(userID, 7)

	return stats
}

func (s *Server) getBroadcastStatsForPeriod(userID uint, start, end time.Time) BroadcastStatsPeriod {
	var stats BroadcastStatsPeriod

	s.db.Model(&database.BroadcastMessage{}).Where("user_id = ? AND created_at >= ? AND created_at < ?", userID, start, end).Count(&stats.Total)
	s.db.Model(&database.BroadcastMessage{}).Where("user_id = ? AND created_at >= ? AND created_at < ? AND status = ?", userID, start, end, "completed").Count(&stats.Completed)
	s.db.Model(&database.BroadcastMessage{}).Where("user_id = ? AND created_at >= ? AND created_at < ? AND status = ?", userID, start, end, "failed").Count(&stats.Failed)
	s.db.Model(&database.BroadcastMessage{}).Where("user_id = ? AND created_at >= ? AND created_at < ? AND status = ?", userID, start, end, "cancelled").Count(&stats.Cancelled)

	// Get total sent and failed counts
	type SumResult struct {
		TotalSent   int64
		TotalFailed int64
	}
	var sumResult SumResult
	s.db.Model(&database.BroadcastMessage{}).Select("COALESCE(SUM(sent_count), 0) as total_sent, COALESCE(SUM(failed_count), 0) as total_failed").Where("user_id = ? AND created_at >= ? AND created_at < ?", userID, start, end).Scan(&sumResult)
	stats.TotalSent = sumResult.TotalSent
	stats.TotalFailed = sumResult.TotalFailed

	return stats
}

func (s *Server) getDailyBroadcastStats(userID uint, days int) []DailyBroadcastStats {
	stats := make([]DailyBroadcastStats, 0, days)
	now := time.Now()

	for i := days - 1; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
		dayEnd := dayStart.AddDate(0, 0, 1)

		dailyStat := DailyBroadcastStats{
			Date: dayStart.Format("2006-01-02"),
		}

		s.db.Model(&database.BroadcastMessage{}).Where("user_id = ? AND created_at >= ? AND created_at < ?", userID, dayStart, dayEnd).Count(&dailyStat.Total)
		s.db.Model(&database.BroadcastMessage{}).Where("user_id = ? AND created_at >= ? AND created_at < ? AND status = ?", userID, dayStart, dayEnd, "completed").Count(&dailyStat.Completed)
		s.db.Model(&database.BroadcastMessage{}).Where("user_id = ? AND created_at >= ? AND created_at < ? AND status = ?", userID, dayStart, dayEnd, "failed").Count(&dailyStat.Failed)

		// Get total sent and failed counts for the day
		type DaySumResult struct {
			TotalSent   int64
			TotalFailed int64
		}
		var daySumResult DaySumResult
		s.db.Model(&database.BroadcastMessage{}).Select("COALESCE(SUM(sent_count), 0) as total_sent, COALESCE(SUM(failed_count), 0) as total_failed").Where("user_id = ? AND created_at >= ? AND created_at < ?", userID, dayStart, dayEnd).Scan(&daySumResult)
		dailyStat.TotalSent = daySumResult.TotalSent
		dailyStat.TotalFailed = daySumResult.TotalFailed

		stats = append(stats, dailyStat)
	}

	return stats
}