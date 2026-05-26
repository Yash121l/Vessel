package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Yash121l/Vessel/internal/store"
)

func listUsers(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		users, err := db.ListUsers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if users == nil {
			users = []*store.User{}
		}
		c.JSON(http.StatusOK, gin.H{"users": users})
	}
}

func createUser(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := currentUser(c)
		var req struct {
			Username string `json:"username" binding:"required"`
			Role     string `json:"role" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		req.Username = strings.TrimSpace(req.Username)
		if err := validateUsername(req.Username); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := validateRole(req.Role); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if !canManageRole(actor.Role, req.Role) {
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot create a user with that role"})
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
				Username: req.Username,
				Role:     req.Role,
			},
			PasswordHash: hash,
		}
		if err := db.CreateUser(user); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		created, _ := db.GetUser(user.ID)
		c.JSON(http.StatusCreated, created)
	}
}

func updateUser(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := currentUser(c)
		target, err := db.GetUser(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if target == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}

		var req struct {
			Role     string `json:"role" binding:"required"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := validateRole(req.Role); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if !canManageRole(actor.Role, target.Role) || !canManageRole(actor.Role, req.Role) {
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot modify that user"})
			return
		}
		if target.ID == actor.ID && req.Role != target.Role {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot change your own role"})
			return
		}

		passwordHash := ""
		if strings.TrimSpace(req.Password) != "" {
			hash, err := passwordHashFromRequest(req.Password)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			passwordHash = hash
		}
		if err := db.UpdateUser(target.ID, req.Role, passwordHash); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if passwordHash != "" {
			_ = db.DeleteUserSessions(target.ID)
		}
		updated, _ := db.GetUser(target.ID)
		c.JSON(http.StatusOK, updated)
	}
}

func deleteUser(db *store.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := currentUser(c)
		target, err := db.GetUser(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if target == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		if target.ID == actor.ID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete your own account"})
			return
		}
		if !canManageRole(actor.Role, target.Role) {
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot delete that user"})
			return
		}
		if target.Role == roleOwner {
			c.JSON(http.StatusBadRequest, gin.H{"error": "owner accounts cannot be deleted"})
			return
		}
		if err := db.DeleteUser(target.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	}
}

func passwordHashFromRequest(password string) (string, error) {
	return passwordHash(password)
}
