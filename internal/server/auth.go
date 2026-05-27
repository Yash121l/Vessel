package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/Yash121l/Vessel/internal/store"
)

const (
	roleOwner    = "owner"
	roleAdmin    = "admin"
	roleOperator = "operator"
	roleViewer   = "viewer"

	settingAdminPasswordHash = "admin_password_hash"
	sessionCookieName        = "vessel_session"
	currentUserKey           = "current_user"
)

var roleRank = map[string]int{
	roleViewer:   10,
	roleOperator: 20,
	roleAdmin:    30,
	roleOwner:    40,
}

func setupStatus(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		configured, err := isAdminConfigured(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"configured": configured})
	}
}

func setupAdmin(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		configured, err := isAdminConfigured(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if configured {
			c.JSON(http.StatusConflict, gin.H{"error": "owner account already configured"})
			return
		}

		var req struct {
			Username string `json:"username"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		username := strings.TrimSpace(req.Username)
		if username == "" {
			username = "admin"
		}
		if err := validateUsername(username); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		hash, err := passwordHash(req.Password)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		user := &store.UserWithPassword{
			User: store.User{
				ID:       uuid.NewString(),
				Username: username,
				Role:     roleOwner,
			},
			PasswordHash: hash,
		}
		if err := db.CreateUser(user); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		created, _ := db.GetUser(user.ID)
		issueSession(c, db, created)
	}
}

func login(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		username := strings.TrimSpace(req.Username)
		if username == "" {
			username = "admin"
		}

		user, err := db.GetUserByUsername(username)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if user == nil {
			if tryLegacyAdminLogin(c, db, username, req.Password) {
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
			return
		}
		_ = db.TouchUserLogin(user.ID)
		issueSession(c, db, &user.User)
	}
}

func logout(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token := sessionToken(c); token != "" {
			_ = db.DeleteSession(hashToken(token))
		}
		clearSessionCookie(c)
		c.JSON(http.StatusOK, gin.H{"status": "logged out"})
	}
}

func me(version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user": currentUser(c), "version": version})
	}
}

func authRequired(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		configured, err := isAdminConfigured(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
		if !configured {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "setup required"})
			c.Abort()
			return
		}
		user, ok := validSession(c, db)
		if ok {
			c.Set(currentUserKey, user)
			c.Next()
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		c.Abort()
	}
}

func requireRole(minRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUser(c)
		if user == nil || !roleAtLeast(user.Role, minRole) {
			c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func requirePermission() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
			if strings.HasPrefix(c.Request.URL.Path, "/api/v1/nginx") ||
				strings.HasPrefix(c.Request.URL.Path, "/api/v1/system") {
				requireRole(roleAdmin)(c)
				return
			}
			c.Next()
			return
		}
		if c.Request.URL.Path == "/api/v1/logout" {
			c.Next()
			return
		}
		if strings.HasPrefix(c.Request.URL.Path, "/api/v1/users") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/v1/system") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/v1/settings") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/v1/nginx") {
			requireRole(roleAdmin)(c)
			return
		}
		requireRole(roleOperator)(c)
	}
}

func currentUser(c *gin.Context) *store.User {
	v, ok := c.Get(currentUserKey)
	if !ok {
		return nil
	}
	user, _ := v.(*store.User)
	return user
}

func isAdminConfigured(db *store.DB) (bool, error) {
	count, err := db.CountUsers()
	if err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	hash, err := db.GetSetting(settingAdminPasswordHash)
	return hash != "", err
}

func issueSession(c *gin.Context, db *store.DB, user *store.User) {
	if user == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found"})
		return
	}
	token, err := randomToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	if err := db.CreateSession(hashToken(token), user.ID, expiresAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = db.CleanupExpiredSessions()
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	})
	c.JSON(http.StatusOK, gin.H{"status": "ok", "user": user})
}

func clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func validSession(c *gin.Context, db *store.DB) (*store.User, bool) {
	token := sessionToken(c)
	if token == "" {
		return nil, false
	}
	user, err := db.GetSessionUser(hashToken(token))
	if err != nil || user == nil {
		return nil, false
	}
	return user, true
}

func sessionToken(c *gin.Context) string {
	if cookie, err := c.Request.Cookie(sessionCookieName); err == nil {
		return cookie.Value
	}
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return ""
}

func tryLegacyAdminLogin(c *gin.Context, db *store.DB, username, password string) bool {
	if username != "admin" {
		return false
	}
	hash, err := db.GetSetting(settingAdminPasswordHash)
	if err != nil || hash == "" {
		return false
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return false
	}
	user := &store.UserWithPassword{
		User: store.User{
			ID:       uuid.NewString(),
			Username: "admin",
			Role:     roleOwner,
		},
		PasswordHash: hash,
	}
	if err := db.CreateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return true
	}
	_ = db.SetSetting(settingAdminPasswordHash, "")
	_ = db.TouchUserLogin(user.ID)
	created, _ := db.GetUser(user.ID)
	issueSession(c, db, created)
	return true
}

func passwordHash(password string) (string, error) {
	if len(password) < 12 {
		return "", fmt.Errorf("password must be at least 12 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func roleAtLeast(role, minRole string) bool {
	return roleRank[role] >= roleRank[minRole]
}

func canManageRole(actorRole, targetRole string) bool {
	if actorRole == roleOwner {
		return true
	}
	return targetRole != roleOwner && roleAtLeast(actorRole, targetRole)
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
