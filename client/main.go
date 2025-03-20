package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"network-monitor-client/bandwidthmonitor"
	"network-monitor-client/config"
	"network-monitor-client/devicemonitor"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/quic-go/quic-go"
)

type BandwidthData struct {
	ClientID              string    `json:"client_id"`
	Alias                 string    `json:"alias"`
	Timestamp             time.Time `json:"timestamp"`
	UploadSpeed           float64   `json:"upload_speed"`
	DownloadSpeed         float64   `json:"download_speed"`
	AvgUploadPacketSize   float64   `json:"avg_upload_packet_size"`
	AvgDownloadPacketSize float64   `json:"avg_download_packet_size"`
}

type Client struct {
	conn          quic.Connection
	monitor       *bandwidthmonitor.BandwidthMonitor
	deviceMonitor *devicemonitor.DeviceMonitor
	failureCount  int
	mutex         sync.Mutex
	isMonitoring  bool
	lastSendTime  time.Time
}

func NewClient(m *bandwidthmonitor.BandwidthMonitor, d *devicemonitor.DeviceMonitor) *Client {
	return &Client{
		monitor:       m,
		deviceMonitor: d,
		isMonitoring:  false,
		lastSendTime:  time.Time{},
	}
}

func (c *Client) startMonitoring() {
	c.monitor.Start()
	c.isMonitoring = true
	log.Printf("开始监控网络带宽")
}

func (c *Client) stopMonitoring() {
	if c.isMonitoring {
		c.monitor.Stop()
		c.isMonitoring = false
		log.Printf("停止监控网络带宽")
	}
}

func (c *Client) connect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.conn != nil {
		c.conn.CloseWithError(0, "reconnecting")
		c.conn = nil
	}

	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"HLD"},
	}

	serverAddr := config.Config.Server.Host + ":" + config.Config.Server.Port
	conn, err := quic.DialAddr(context.Background(), serverAddr, tlsConf, nil)
	if err != nil {
		log.Printf("连接服务器失败: %v", err)
		return err
	}

	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		log.Printf("连接测试失败: %v", err)
		conn.CloseWithError(0, "connection test failed")
		return err
	}
	stream.Close()

	c.conn = conn
	c.failureCount = 0
	log.Printf("成功连接到服务器")
	return nil
}

func (c *Client) sendData(data BandwidthData) error {
	c.mutex.Lock()
	if !c.isMonitoring || c.conn == nil {
		c.mutex.Unlock()
		return nil
	}

	// 确保发送间隔至少为500ms
	now := time.Now()
	if now.Sub(c.lastSendTime) < 500*time.Millisecond {
		c.mutex.Unlock()
		return nil
	}

	conn := c.conn
	c.mutex.Unlock()

	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		c.handleFailure()
		return err
	}
	defer stream.Close()

	if err := json.NewEncoder(stream).Encode(data); err != nil {
		c.handleFailure()
		return err
	}

	c.mutex.Lock()
	c.failureCount = 0
	c.lastSendTime = now
	c.mutex.Unlock()

	log.Printf("数据发送成功 - 上行: %.2f Kbps, 下行: %.2f Kbps",
		data.UploadSpeed*8/1000,
		data.DownloadSpeed*8/1000)
	return nil
}

func (c *Client) handleFailure() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.failureCount++
	log.Printf("发送失败次数: %d/5", c.failureCount)

	if c.failureCount >= 5 {
		log.Printf("连续5次发送失败，停止监控")
		c.stopMonitoring()
		if c.conn != nil {
			c.conn.CloseWithError(0, "connection failed")
			c.conn = nil
		}
	}
}

func (c *Client) stop() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.stopMonitoring()
	if c.conn != nil {
		c.conn.CloseWithError(0, "client shutdown")
		c.conn = nil
	}
}

func (c *Client) tryReconnect() bool {
	log.Printf("尝试重新连接服务器...")

	if err := c.connect(); err != nil {
		log.Printf("连接失败: %v", err)
		return false
	}

	// 启动监控以获取实际数据
	c.startMonitoring()

	// 等待2秒钟收集初始数据
	time.Sleep(2 * time.Second)

	for i := 0; i < 5; i++ {
		stats := c.monitor.GetStats()
		if stats == nil {
			log.Printf("测试数据包 %d/5 发送失败: 无法获取带宽数据", i+1)
			c.mutex.Lock()
			c.stopMonitoring()
			if c.conn != nil {
				c.conn.CloseWithError(0, "test failed")
				c.conn = nil
			}
			c.mutex.Unlock()
			return false
		}

		data := BandwidthData{
			ClientID:              config.Config.Client.ID,
			Alias:                 config.Config.Client.Alias,
			Timestamp:             time.Now(),
			UploadSpeed:           stats.UploadSpeed,
			DownloadSpeed:         stats.DownloadSpeed,
			AvgUploadPacketSize:   stats.AvgUploadPacketSize,
			AvgDownloadPacketSize: stats.AvgDownloadPacketSize,
		}

		if err := c.sendData(data); err != nil {
			log.Printf("测试数据包 %d/5 发送失败", i+1)
			c.mutex.Lock()
			c.stopMonitoring()
			if c.conn != nil {
				c.conn.CloseWithError(0, "test failed")
				c.conn = nil
			}
			c.mutex.Unlock()
			return false
		}
		time.Sleep(time.Second)
	}

	log.Printf("重连成功，继续监控")
	return true
}

func main() {
	if err := config.Init(); err != nil {
		log.Fatalf("初始化配置失败: %v", err)
	}

	log.Printf("正在初始化网络监控客户端...")
	log.Printf("采样间隔: %v，上报间隔: %v",
		config.Config.Monitor.SampleInterval,
		config.Config.Monitor.ReportInterval)

	deviceMonitor := devicemonitor.NewDeviceMonitor("")
	defer deviceMonitor.Close()

	m := bandwidthmonitor.NewBandwidthMonitor(deviceMonitor, config.Config.Monitor.SampleInterval)
	client := NewClient(m, deviceMonitor)
	defer client.stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	dataTicker := time.NewTicker(config.Config.Monitor.ReportInterval)
	reconnectTicker := time.NewTicker(config.Config.Server.RetryInterval)
	defer dataTicker.Stop()
	defer reconnectTicker.Stop()

	// 首次连接
	if err := client.connect(); err != nil {
		log.Printf("首次连接失败，等待重试")
	} else {
		// 只有在首次连接成功时才启动监控
		client.startMonitoring()
		log.Printf("客户端启动成功，ID: %s, 别名: %s",
			config.Config.Client.ID, config.Config.Client.Alias)
	}

	for {
		select {
		case <-dataTicker.C:
			if !client.isMonitoring {
				continue
			}

			stats := m.GetStats()
			if stats == nil {
				log.Printf("获取带宽统计数据失败")
				continue
			}

			data := BandwidthData{
				ClientID:              config.Config.Client.ID,
				Alias:                 config.Config.Client.Alias,
				Timestamp:             time.Now(),
				UploadSpeed:           stats.UploadSpeed,
				DownloadSpeed:         stats.DownloadSpeed,
				AvgUploadPacketSize:   stats.AvgUploadPacketSize,
				AvgDownloadPacketSize: stats.AvgDownloadPacketSize,
			}

			if err := client.sendData(data); err != nil {
				log.Printf("发送数据失败: %v", err)
			}

		case <-reconnectTicker.C:
			if !client.isMonitoring {
				client.tryReconnect()
			}

		case <-quit:
			log.Println("正在关闭客户端...")
			return
		}
	}
}
