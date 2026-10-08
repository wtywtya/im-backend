package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"im-project/service"
	"im-project/utils"
	"log"
	"net/http"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func WsHandler(c *gin.Context) {
	tokenStr := c.Query("token")
	if tokenStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "缺失 Token"})
		return
	}
	uid, err := utils.ParseToken(tokenStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的 Token"})
		return
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket 升级失败: %v", err)
		return
	}
	service.RegisterClient(uid, conn)
}
