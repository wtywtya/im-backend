package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"im-project/global"
	"im-project/models"
)

func GetMessages(c *gin.Context) {
	value, exists := c.Get("uid")
	uid, ok := value.(uint)
	if !exists || !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "用户身份信息异常"})
		return
	}

	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := parsePositiveInt(c.Query("page_size"), 20)
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize
	var messages []models.Message

	err := global.DB.
		Where("sender_id = ? OR receiver_id = ?", uid, uid).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&messages).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询消息失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"page":      page,
		"page_size": pageSize,
		"messages":  messages,
	})
}

func parsePositiveInt(value string, defaultValue int) int {
	result, err := strconv.Atoi(value)
	if err != nil || result <= 0 {
		return defaultValue
	}
	return result
}
