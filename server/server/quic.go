package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"sync"
	"time"

	"network-monitor-server/database"

	"github.com/quic-go/quic-go"
)

// BandwidthData 带宽数据结构
type BandwidthData struct {
	Timestamp             time.Time `json:"timestamp"`
	UploadSpeed           float64   `json:"upload_speed"`
	DownloadSpeed         float64   `json:"download_speed"`
	AvgUploadPacketSize   float64   `json:"avg_upload_packet_size"`
	AvgDownloadPacketSize float64   `json:"avg_download_packet_size"`
}

type QuicServer struct {
	listener    *quic.Listener
	addr        string
	started     bool
	ctx         context.Context
	cancel      context.CancelFunc
	connections sync.WaitGroup
}

func NewQuicServer(addr string) *QuicServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &QuicServer{
		addr:    addr,
		started: false,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// 生成TLS配置
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"HLD"},
	}
}

func (s *QuicServer) Start() error {
	var err error
	s.listener, err = quic.ListenAddr(s.addr, generateTLSConfig(), nil)
	if err != nil {
		return fmt.Errorf("failed to start QUIC server: %v", err)
	}
	s.started = true
	log.Printf("QUIC server started on %s", s.addr)

	// 使用 context 控制接受连接的循环
	for {
		select {
		case <-s.ctx.Done():
			return nil
		default:
			conn, err := s.listener.Accept(s.ctx)
			if err != nil {
				if s.ctx.Err() != nil {
					// 正常关闭情况
					return nil
				}
				log.Printf("Failed to accept connection: %v", err)
				continue
			}
			s.connections.Add(1)
			go func() {
				defer s.connections.Done()
				s.handleConnection(conn)
			}()
		}
	}
}

func (s *QuicServer) Stop() error {
	if !s.started {
		return nil
	}

	// 取消 context，通知所有 goroutine 退出
	s.cancel()

	// 关闭监听器
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			log.Printf("Error closing QUIC listener: %v", err)
		}
	}

	// 等待所有连接处理完成
	s.connections.Wait()
	s.started = false
	return nil
}

func (s *QuicServer) handleConnection(conn quic.Connection) {
	log.Printf("New connection from %s", conn.RemoteAddr())
	defer conn.CloseWithError(0, "connection closed")

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			stream, err := conn.AcceptStream(s.ctx)
			if err != nil {
				if s.ctx.Err() != nil {
					return
				}
				log.Printf("Failed to accept stream: %v", err)
				return
			}

			s.handleStream(stream)
		}
	}
}

func (s *QuicServer) handleStream(stream quic.Stream) {
	defer stream.Close()

	// 设置读取超时
	deadline := time.Now().Add(5 * time.Second)
	if err := stream.SetReadDeadline(deadline); err != nil {
		log.Printf("Failed to set read deadline: %v", err)
		return
	}

	// 读取数据
	data, err := io.ReadAll(stream)
	if err != nil {
		if err != io.EOF {
			log.Printf("Failed to read from stream: %v", err)
		}
		return
	}

	// 检查数据是否为空
	if len(data) == 0 {
		return
	}

	var bandwidthData BandwidthData
	if err := json.Unmarshal(data, &bandwidthData); err != nil {
		log.Printf("Failed to unmarshal bandwidth data: %v, raw data: %s", err, string(data))
		return
	}

	// 验证数据有效性
	if bandwidthData.Timestamp.IsZero() {
		log.Printf("Invalid bandwidth data: timestamp is zero")
		return
	}

	// 使用新的数据库操作方法保存带宽数据
	err = database.SaveBandwidthData(
		bandwidthData.Timestamp,
		bandwidthData.UploadSpeed,
		bandwidthData.DownloadSpeed,
		bandwidthData.AvgUploadPacketSize,
		bandwidthData.AvgDownloadPacketSize)
	if err != nil {
		log.Printf("Failed to save bandwidth data: %v", err)
		return
	}

	log.Printf("Successfully saved bandwidth data from stream")
}

// 关闭连接
func closeConnection(conn quic.Connection) {
	conn.CloseWithError(0, "QUIC连接已关闭")
}
