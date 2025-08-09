package auth

import (
	"errors"
	"time"

	"gowa-broadcast/internal/database"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	db        *gorm.DB
	jwtSecret []byte
}

type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token     string           `json:"token"`
	ExpiresAt time.Time        `json:"expires_at"`
	User      UserResponse     `json:"user"`
}

type UserResponse struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
	Active   bool   `json:"active"`
}

type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	FullName string `json:"full_name" binding:"required"`
	Role     string `json:"role" binding:"required,oneof=admin user"`
}

type UpdateUserRequest struct {
	Email    string `json:"email" binding:"omitempty,email"`
	FullName string `json:"full_name"`
	Role     string `json:"role" binding:"omitempty,oneof=admin user"`
	Active   *bool  `json:"active"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
}

func NewAuthService(db *gorm.DB, jwtSecret string) *AuthService {
	return &AuthService{
		db:        db,
		jwtSecret: []byte(jwtSecret),
	}
}

// Login authenticates user and returns JWT token
func (a *AuthService) Login(req LoginRequest) (*LoginResponse, error) {
	var user database.User
	if err := a.db.Where("username = ? AND active = ?", req.Username, true).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid credentials")
		}
		return nil, err
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	// Generate JWT token
	expiresAt := time.Now().Add(24 * time.Hour) // 24 hours
	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "gowa-broadcast",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(a.jwtSecret)
	if err != nil {
		return nil, err
	}

	return &LoginResponse{
		Token:     tokenString,
		ExpiresAt: expiresAt,
		User: UserResponse{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			FullName: user.FullName,
			Role:     user.Role,
			Active:   user.Active,
		},
	}, nil
}

// ValidateToken validates JWT token and returns claims
func (a *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return a.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		// Check if user is still active
		var user database.User
		if err := a.db.Where("id = ? AND active = ?", claims.UserID, true).First(&user).Error; err != nil {
			return nil, errors.New("user not found or inactive")
		}
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// CreateUser creates a new user (admin only)
func (a *AuthService) CreateUser(req CreateUserRequest) (*UserResponse, error) {
	// Check if username or email already exists
	var existingUser database.User
	if err := a.db.Where("username = ? OR email = ?", req.Username, req.Email).First(&existingUser).Error; err == nil {
		return nil, errors.New("username or email already exists")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := database.User{
		Username: req.Username,
		Email:    req.Email,
		Password: string(hashedPassword),
		FullName: req.FullName,
		Role:     req.Role,
		Active:   true,
	}

	if err := a.db.Create(&user).Error; err != nil {
		return nil, err
	}

	return &UserResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		FullName: user.FullName,
		Role:     user.Role,
		Active:   user.Active,
	}, nil
}

// GetUsers returns list of users (admin only)
func (a *AuthService) GetUsers(limit, offset int) ([]UserResponse, int64, error) {
	var users []database.User
	var total int64

	// Get total count
	a.db.Model(&database.User{}).Count(&total)

	// Get users with pagination
	query := a.db.Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&users).Error; err != nil {
		return nil, 0, err
	}

	response := make([]UserResponse, len(users))
	for i, user := range users {
		response[i] = UserResponse{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			FullName: user.FullName,
			Role:     user.Role,
			Active:   user.Active,
		}
	}

	return response, total, nil
}

// GetUser returns user by ID
func (a *AuthService) GetUser(userID uint) (*UserResponse, error) {
	var user database.User
	if err := a.db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	return &UserResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		FullName: user.FullName,
		Role:     user.Role,
		Active:   user.Active,
	}, nil
}

// UpdateUser updates user information
func (a *AuthService) UpdateUser(userID uint, req UpdateUserRequest) (*UserResponse, error) {
	var user database.User
	if err := a.db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	// Check if email already exists (if changing email)
	if req.Email != "" && req.Email != user.Email {
		var existingUser database.User
		if err := a.db.Where("email = ? AND id != ?", req.Email, userID).First(&existingUser).Error; err == nil {
			return nil, errors.New("email already exists")
		}
		user.Email = req.Email
	}

	if req.FullName != "" {
		user.FullName = req.FullName
	}
	if req.Role != "" {
		user.Role = req.Role
	}
	if req.Active != nil {
		user.Active = *req.Active
	}

	if err := a.db.Save(&user).Error; err != nil {
		return nil, err
	}

	return &UserResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		FullName: user.FullName,
		Role:     user.Role,
		Active:   user.Active,
	}, nil
}

// DeleteUser deletes user (admin only)
func (a *AuthService) DeleteUser(userID uint) error {
	// Don't allow deleting the last admin
	var user database.User
	if err := a.db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return err
	}

	if user.Role == "admin" {
		var adminCount int64
		a.db.Model(&database.User{}).Where("role = ? AND active = ?", "admin", true).Count(&adminCount)
		if adminCount <= 1 {
			return errors.New("cannot delete the last admin user")
		}
	}

	return a.db.Delete(&user).Error
}

// ChangePassword changes user password
func (a *AuthService) ChangePassword(userID uint, req ChangePasswordRequest) error {
	var user database.User
	if err := a.db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return err
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		return errors.New("current password is incorrect")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.Password = string(hashedPassword)
	return a.db.Save(&user).Error
}

// GetUserByID returns user from database
func (a *AuthService) GetUserByID(userID uint) (*database.User, error) {
	var user database.User
	if err := a.db.First(&user, userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}