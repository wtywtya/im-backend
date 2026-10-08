package routers

import (
	"github.com/gin-gonic/gin"
	"im-project/controllers"
	"im-project/middleware"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()
	r.Use(func(c *gin.Context) { c.Writer.Header().Set("Access-Control-Allow-Origin", "*"); c.Next() })
	r.POST("/register", controllers.Register)
	r.POST("/login", controllers.Login)
	protected := r.Group("/")
	protected.Use(middleware.AuthRequired)
	protected.GET("/messages", controllers.GetMessages)
	r.GET("/ws", controllers.WsHandler)
	return r
}
