package server

import (
	"net/http"
	"strconv"

	"gowa-broadcast/internal/auth"
	"gowa-broadcast/internal/middleware"

	"github.com/gin-gonic/gin"
)

// AuthHandlers contains all authentication related handlers
type AuthHandlers struct {
	authService *auth.AuthService
}

// NewAuthHandlers creates a new AuthHandlers instance
func NewAuthHandlers(authService *auth.AuthService) *AuthHandlers {
	return &AuthHandlers{
		authService: authService,
	}
}

// Login handles user login
func (h *AuthHandlers) Login(c *gin.Context) {
	var req auth.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	response, err := h.authService.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// CreateUser handles user creation (admin only)
func (h *AuthHandlers) CreateUser(c *gin.Context) {
	var req auth.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	currentUserID, _ := middleware.GetCurrentUserID(c)
	user, err := h.authService.CreateUser(req, currentUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User created successfully",
		"user": gin.H{
			"id":        user.ID,
			"username":  user.Username,
			"email":     user.Email,
			"full_name": user.FullName,
			"role":      user.Role,
			"active":    user.Active,
			"created_at": user.CreatedAt,
		},
	})
}

// GetUsers handles getting list of users (admin only)
func (h *AuthHandlers) GetUsers(c *gin.Context) {
	users, err := h.authService.GetUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"users": users})
}

// GetUser handles getting user details
func (h *AuthHandlers) GetUser(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	user, err := h.authService.GetUserByID(uint(userID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user})
}

// UpdateUser handles updating user information
func (h *AuthHandlers) UpdateUser(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req auth.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	currentUserID, _ := middleware.GetCurrentUserID(c)
	user, err := h.authService.UpdateUser(uint(userID), req, currentUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User updated successfully",
		"user": gin.H{
			"id":        user.ID,
			"username":  user.Username,
			"email":     user.Email,
			"full_name": user.FullName,
			"role":      user.Role,
			"active":    user.Active,
			"updated_at": user.UpdatedAt,
		},
	})
}

// DeleteUser handles user deletion (admin only)
func (h *AuthHandlers) DeleteUser(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	currentUserID, _ := middleware.GetCurrentUserID(c)
	err = h.authService.DeleteUser(uint(userID), currentUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// ChangePassword handles password change
func (h *AuthHandlers) ChangePassword(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req auth.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	currentUserID, _ := middleware.GetCurrentUserID(c)
	err = h.authService.ChangePassword(uint(userID), req, currentUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

// GetProfile handles getting current user profile
func (h *AuthHandlers) GetProfile(c *gin.Context) {
	currentUserID, _ := middleware.GetCurrentUserID(c)
	user, err := h.authService.GetUserByID(currentUserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user})
}

// UpdateProfile handles updating current user profile
func (h *AuthHandlers) UpdateProfile(c *gin.Context) {
	var req auth.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	currentUserID, _ := middleware.GetCurrentUserID(c)
	user, err := h.authService.UpdateUser(currentUserID, req, currentUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Profile updated successfully",
		"user": gin.H{
			"id":        user.ID,
			"username":  user.Username,
			"email":     user.Email,
			"full_name": user.FullName,
			"role":      user.Role,
			"active":    user.Active,
			"updated_at": user.UpdatedAt,
		},
	})
}

// ChangeMyPassword handles current user password change
func (h *AuthHandlers) ChangeMyPassword(c *gin.Context) {
	var req auth.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	currentUserID, _ := middleware.GetCurrentUserID(c)
	err := h.authService.ChangePassword(currentUserID, req, currentUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

// ValidateToken handles token validation
func (h *AuthHandlers) ValidateToken(c *gin.Context) {
	// If we reach here, the token is valid (middleware already validated it)
	currentUserID, _ := middleware.GetCurrentUserID(c)
	currentUsername, _ := middleware.GetCurrentUsername(c)
	currentUserRole, _ := middleware.GetCurrentUserRole(c)

	c.JSON(http.StatusOK, gin.H{
		"valid": true,
		"user": gin.H{
			"id":       currentUserID,
			"username": currentUsername,
			"role":     currentUserRole,
		},
	})
}