package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"network-monitor-client/config"

	"github.com/quic-go/quic-go"
)

// BandwidthData 带宽数据结构
type BandwidthData struct {
	ClientID              string    `json:"client_id"`
	Alias                 string    `json:"alias"`
	Timestamp             time.Time `json:"timestamp"`
	UploadSpeed           float64   `json:"upload_speed"`
	DownloadSpeed         float64   `json:"download_speed"`
	AvgUploadPacketSize   float64   `json:"avg_upload_packet_size"`
	AvgDownloadPacketSize float64   `json:"avg_download_packet_size"`
}

// 创建QUIC客户端
func CreateQUICClient() (quic.Connection, error) {
	// 创建TLS配置
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"HLD"},
	}

	// 创建QUIC客户端
	client, err := quic.DialAddr(context.Background(), config.Config.Server.Host+":"+config.Config.Server.Port, tlsConf, nil)
	if err != nil {
		log.Fatalf("创建QUIC客户端失败: %v", err)
		return nil, errors.New("创建QUIC客户端失败")
	}
	return client, nil
}

// 发送数据
func SendData(client quic.Connection, data []byte) error {
	stream, err := client.OpenStreamSync(context.Background())
	if err != nil {
		log.Fatalf("创建数据流失败: %v", err)
		return errors.New("创建数据流失败")
	}
	defer stream.Close()

	// 发送数据
	_, err = stream.Write(data)
	if err != nil {
		log.Fatalf("发送数据失败: %v", err)
		return errors.New("发送数据失败")
	}

	return nil
}

// SendBandwidthData 发送带宽数据
func SendBandwidthData(client quic.Connection, data *BandwidthData) error {
	// 确保数据中包含客户端标识
	if data.ClientID == "" {
		data.ClientID = config.Config.Client.ID
	}
	if data.Alias == "" {
		data.Alias = config.Config.Client.Alias
	}

	// 将数据转换为JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("数据序列化失败: %v", err)
	}

	// 发送数据
	stream, err := client.OpenStreamSync(context.Background())
	if err != nil {
		return fmt.Errorf("创建数据流失败: %v", err)
	}
	defer stream.Close()

	// 发送数据
	if _, err := stream.Write(jsonData); err != nil {
		return fmt.Errorf("发送数据失败: %v", err)
	}

	return nil
}

// 接收数据
func ReceiveData(client quic.Connection) ([]byte, error) {
	stream, err := client.AcceptStream(context.Background())
	if err != nil {
		log.Fatalf("接收数据流失败: %v", err)
		return nil, errors.New("接收数据流失败")
	}
	defer stream.Close()

	// 接收数据
	data := make([]byte, 1024)
	_, err = stream.Read(data)
	if err != nil {
		log.Fatalf("接收数据失败: %v", err)
		return nil, errors.New("接收数据失败")
	}

	return data, nil
}
