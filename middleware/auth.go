package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"im-project/utils"
)

func AuthRequired(c *gin.Context) {
	authorization := c.GetHeader("Authorization")
	parts := strings.SplitN(authorization, " ", 2)

	if len(parts) != 2 || parts[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少或错误的 Authorization"})
		c.Abort()
		return
	}

	uid, err := utils.ParseToken(parts[1])
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效或过期的 Token"})
		c.Abort()
		return
	}

	c.Set("uid", uid)
	c.Next()
}
