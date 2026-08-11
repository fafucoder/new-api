package middleware

import (
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// PlaygroundUserToToken bridges a session-authenticated dashboard user into
// the token-style context required by the relay pipeline (Distribute / RelayTask).
// Must run AFTER UserAuth() and BEFORE Distribute().
//
// Honors the optional `New-API-Group` header to switch among the user's usable
// groups, mirroring the /pg/chat/completions playground flow.
func PlaygroundUserToToken() func(c *gin.Context) {
	return func(c *gin.Context) {
		userId := c.GetInt("id")
		if userId == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "user not authenticated"})
			c.Abort()
			return
		}

		userCache, err := model.GetUserCache(userId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
			c.Abort()
			return
		}
		userCache.WriteContext(c)

		usingGroup := userCache.Group
		if hdr := c.GetHeader("New-API-Group"); hdr != "" {
			if !service.GroupInUserUsableGroups(userCache.Group, hdr) && hdr != userCache.Group {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "group not allowed"})
				c.Abort()
				return
			}
			usingGroup = hdr
		}
		common.SetContextKey(c, constant.ContextKeyUserId, userId)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, usingGroup)
		// Marks this as a playground-style relay so quota/billing helpers
		// treat the lack of a real token as expected (see relay_info.go).
		c.Set("is_playground", true)

		tempToken := &model.Token{
			UserId:         userId,
			Name:           fmt.Sprintf("playground-%s", usingGroup),
			Group:          usingGroup,
			UnlimitedQuota: false,
			RemainQuota:    userCache.Quota,
		}
		if err := SetupContextForToken(c, tempToken); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
			c.Abort()
			return
		}
		c.Next()
	}
}
