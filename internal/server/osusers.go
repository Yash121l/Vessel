package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Yash121l/Vessel/internal/system"
)

// listOSUsers returns all non-system OS users (UID >= 1000) plus root.
// Pass ?all=true to include system accounts.
func listOSUsers() gin.HandlerFunc {
	return func(c *gin.Context) {
		includeSystem := c.Query("all") == "true"
		users, err := system.ListOSUsers(includeSystem)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if users == nil {
			users = []system.OSUser{}
		}
		c.JSON(http.StatusOK, gin.H{"users": users})
	}
}

// getOSUser returns a single OS user by username.
func getOSUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.Param("username")
		if err := validateOSUsername(username); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		u, err := system.GetOSUser(username)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if u == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "OS user not found"})
			return
		}
		c.JSON(http.StatusOK, u)
	}
}

// createOSUser creates a new Linux user account.
func createOSUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req system.CreateOSUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		req.Username = strings.TrimSpace(req.Username)
		if err := validateOSUsername(req.Username); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.Shell != "" {
			if err := validateShellPath(req.Shell); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		if req.HomeDir != "" {
			if err := validateAbsPath(req.HomeDir); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		for _, g := range req.Groups {
			if err := validateOSUsername(g); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group name: " + g})
				return
			}
		}

		u, err := system.CreateOSUser(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, u)
	}
}

// updateOSUser modifies an existing Linux user account.
func updateOSUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.Param("username")
		if err := validateOSUsername(username); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Verify user exists
		existing, err := system.GetOSUser(username)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if existing == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "OS user not found"})
			return
		}

		var req system.UpdateOSUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.Shell != "" {
			if err := validateShellPath(req.Shell); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		for _, g := range req.Groups {
			if err := validateOSUsername(g); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group name: " + g})
				return
			}
		}
		if req.Lock && req.Unlock {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot lock and unlock simultaneously"})
			return
		}

		u, err := system.UpdateOSUser(username, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, u)
	}
}

// deleteOSUser removes a Linux user account.
// Pass ?remove_home=true to also delete the home directory.
func deleteOSUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.Param("username")
		if err := validateOSUsername(username); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// Safety: never allow deleting root
		if username == "root" {
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot delete the root account"})
			return
		}

		existing, err := system.GetOSUser(username)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if existing == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "OS user not found"})
			return
		}

		removeHome := c.Query("remove_home") == "true"
		if err := system.DeleteOSUser(username, removeHome); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "deleted", "username": username})
	}
}

// listOSGroups returns all groups from /etc/group.
func listOSGroups() gin.HandlerFunc {
	return func(c *gin.Context) {
		groups, err := system.ListOSGroups()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if groups == nil {
			groups = []string{}
		}
		c.JSON(http.StatusOK, gin.H{"groups": groups})
	}
}
