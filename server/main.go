package main

import (
	"context"
	"log"
	"network-monitor-server/config"
	"network-monitor-server/database"
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

	// 创建一个用于通知goroutine退出的通道
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 创建并启动QUIC服务器
	quicServer := server.NewQuicServer(config.Config.Server.Host + ":" + config.Config.Server.Port)
	go func() {
		if err := quicServer.Start(); err != nil {
			log.Printf("QUIC服务器启动失败: %v", err)
			quit <- syscall.SIGTERM
		}
	}()

	// 等待中断信号
	<-quit
	log.Println("正在关闭服务器...")

	// 创建一个用于超时控制的context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 优雅关闭QUIC服务器
	if err := quicServer.Stop(); err != nil {
		log.Printf("QUIC服务器关闭出错: %v", err)
	}

	// 等待所有连接处理完成
	<-ctx.Done()
	log.Println("服务器已关闭")
}
