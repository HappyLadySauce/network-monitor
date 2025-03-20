package bandwidthmonitor

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

// WaitForInterrupt 等待中断信号（如Ctrl+C）以优雅退出
func WaitForInterrupt() {
	// 创建信号通道
	sigChan := make(chan os.Signal, 1)

	// 监听中断信号
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	sig := <-sigChan
	log.Printf("收到信号 %v，程序正在退出...", sig)
}
