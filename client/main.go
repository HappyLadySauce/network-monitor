package main

import (
	"log"
	"network-monitor/bandwidthmonitor"
	"network-monitor/client"
	"network-monitor/config"
	"network-monitor/devicemonitor"
	"time"
)

func main() {
	// 初始化配置
	config.InitConfig()

	log.Println("开始高精度网络带宽监控...")

	// 创建设备监控器
	deviceMonitor := devicemonitor.NewDeviceMonitor("")
	defer deviceMonitor.Close()

	// 创建带宽监控器
	bandwidthMonitor := bandwidthmonitor.NewBandwidthMonitor(deviceMonitor, config.Config.Monitor.SampleInterval)

	// 输出监控设置信息
	log.Printf("采样间隔: %v，上报间隔: %v",
		config.Config.Monitor.SampleInterval,
		config.Config.Monitor.ReportInterval)

	// 启动监控
	bandwidthMonitor.Start()

	// 创建QUIC客户端
	quicClient, err := client.CreateQUICClient()
	if err != nil {
		log.Fatalf("创建QUIC客户端失败: %v", err)
	}
	defer quicClient.CloseWithError(0, "正常关闭")

	// 创建定时器，定期发送数据
	reportTicker := time.NewTicker(config.Config.Monitor.ReportInterval)
	defer reportTicker.Stop()

	// 创建一个goroutine用于定期发送数据
	go func() {
		for {
			select {
			case <-reportTicker.C:
				// 获取当前带宽统计数据
				stats := bandwidthMonitor.GetStats()

				// 创建带宽数据对象
				data := &client.BandwidthData{
					Timestamp:             time.Now(),
					UploadSpeed:           stats.UploadSpeed,
					DownloadSpeed:         stats.DownloadSpeed,
					AvgUploadPacketSize:   stats.AvgUploadPacketSize,
					AvgDownloadPacketSize: stats.AvgDownloadPacketSize,
				}

				// 发送数据
				if err := client.SendBandwidthData(quicClient, data); err != nil {
					log.Printf("发送带宽数据失败: %v", err)
				}
			}
		}
	}()

	// 保持程序运行，使用信号处理优雅退出
	bandwidthmonitor.WaitForInterrupt()
}
