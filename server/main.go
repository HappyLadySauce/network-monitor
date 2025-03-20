package main

import (
	"context"
	"log"
	"net/http"
	"network-monitor-server/config"
	"network-monitor-server/database"
	"network-monitor-server/router"
	"network-monitor-server/server"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// 初始化配置
	config.InitConfig()

	// 初始化数据库
	database.InitDB()
	defer database.Close()

	// 初始化路由
	ginRouter := router.InitRouter()

	// 创建一个用于通知goroutine退出的通道
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 创建并启动QUIC服务器
	quicServer := server.NewQuicServer(config.Config.Server.Host + ":" + config.Config.Server.Port)
	go func() {
		if err := quicServer.Start(); err != nil {
			log.Fatalf("QUIC服务器启动失败: %v", err)
		}
	}()

	// 创建HTTP服务器
	httpServer := &http.Server{
		Addr:    ":" + config.Config.Gin.Port,
		Handler: ginRouter,
	}

	// 在goroutine中启动HTTP服务器
	go func() {
		log.Printf("HTTP服务器已启动,监听端口: %s", config.Config.Gin.Port)
		if err := httpServer.ListenAndServe(); err != nil {
			if err.Error() != "http: Server closed" {
				log.Printf("HTTP服务器错误: %v", err)
			}
		}
	}()

	// 等待中断信号
	<-quit
	log.Println("正在关闭服务器...")

	// 创建一个用于超时控制的context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 优雅关闭HTTP服务器
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP服务器关闭出错: %v", err)
	}

	// 关闭QUIC服务器
	if err := quicServer.Stop(); err != nil {
		log.Printf("QUIC服务器关闭出错: %v", err)
	}

	// 等待所有连接处理完成
	<-ctx.Done()
	log.Println("服务器已关闭")
}
