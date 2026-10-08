package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/go-redis/redis/v8"
	"im-project/global"
	"im-project/routers"
	"im-project/service"
	"im-project/utils"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	port := flag.String("port", "8080", "服务器运行端口")
	flag.Parse()
	if err := utils.ValidateJWTConfig(); err != nil {
		log.Fatal(err)
	}

	global.InitMySQL()

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	global.RDB = redis.NewClient(&redis.Options{Addr: redisAddr})
	go service.StartRedisListener()
	r := routers.SetupRouter()
	addr := fmt.Sprintf(":%s", *port)
	fmt.Printf("启动服务器，监听端口: %s\n", addr)
	server := &http.Server{Addr: addr, Handler: r}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP服务器停止: %v", err)
		}
	case <-stop:
		log.Println("收到关闭信号，正在停止服务器...")
		service.Shutdown()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP服务器优雅退出失败: %v", err)
		}
	}
}
