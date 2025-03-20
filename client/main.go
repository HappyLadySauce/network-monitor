package main

import (
	"flag"
	"log"
	"network-monitor/devicemonitor"
	"network-monitor/bandwidthmonitor"
	"time"
)

func main() {
	// 定义命令行参数
	interval := flag.Int("interval", 500, "带宽采样间隔(毫秒)，默认500ms")

	flag.Parse()

	log.Println("开始高精度网络带宽监控...")

	// 创建设备监控器
	deviceMonitor := devicemonitor.NewDeviceMonitor("")
	defer deviceMonitor.Close()

	// 创建带宽监控器
	bandwidthMonitor := bandwidthmonitor.NewBandwidthMonitor(deviceMonitor, time.Duration(*interval)*time.Millisecond)

	// 输出监控设置信息
	log.Printf("采样间隔: %dms，监控周期: %d秒", *interval, time.Second/bandwidthMonitor.GetInterval()*2/5)

	// 启动监控
	bandwidthMonitor.Start()

	// 保持程序运行，使用信号处理优雅退出
	bandwidthmonitor.WaitForInterrupt()
}
