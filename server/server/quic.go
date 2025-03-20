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
	ClientID              string    `json:"client_id"`
	Alias                 string    `json:"alias"`
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
	clients     sync.Map // 存储活跃的客户端连接
}

type ClientConnection struct {
	ID       string
	Alias    string
	Conn     quic.Connection
	LastSeen time.Time
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

	// 启动客户端状态检查
	go s.checkClientsStatus()

	// 使用 context 控制接受连接的循环
	for {
		select {
		case <-s.ctx.Done():
			return nil
		default:
			conn, err := s.listener.Accept(s.ctx)
			if err != nil {
				if s.ctx.Err() != nil {
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

	s.cancel()

	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			log.Printf("Error closing QUIC listener: %v", err)
		}
	}

	// 关闭所有客户端连接
	s.clients.Range(func(key, value interface{}) bool {
		if client, ok := value.(*ClientConnection); ok {
			client.Conn.CloseWithError(0, "server shutdown")
		}
		return true
	})

	s.connections.Wait()
	s.started = false
	return nil
}

func (s *QuicServer) handleConnection(conn quic.Connection) {
	var clientID, alias string
	defer func() {
		if clientID != "" {
			s.clients.Delete(clientID)
			log.Printf("Client %s (%s) disconnected", clientID, alias)
		}
		conn.CloseWithError(0, "connection closed")
	}()

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

			s.handleStream(stream, conn, &clientID, &alias)
		}
	}
}

func (s *QuicServer) handleStream(stream quic.Stream, conn quic.Connection, clientID *string, alias *string) {
	defer stream.Close()

	deadline := time.Now().Add(5 * time.Second)
	if err := stream.SetReadDeadline(deadline); err != nil {
		log.Printf("Failed to set read deadline: %v", err)
		return
	}

	data, err := io.ReadAll(stream)
	if err != nil {
		if err != io.EOF {
			log.Printf("Failed to read from stream: %v", err)
		}
		return
	}

	if len(data) == 0 {
		return
	}

	var bandwidthData BandwidthData
	if err := json.Unmarshal(data, &bandwidthData); err != nil {
		log.Printf("Failed to unmarshal bandwidth data: %v, raw data: %s", err, string(data))
		return
	}

	// 验证和更新客户端信息
	if *clientID == "" {
		*clientID = bandwidthData.ClientID
		*alias = bandwidthData.Alias

		// 注册客户端
		if err := database.RegisterClient(*clientID, *alias); err != nil {
			log.Printf("Failed to register client: %v", err)
			return
		}

		// 存储客户端连接信息
		s.clients.Store(*clientID, &ClientConnection{
			ID:       *clientID,
			Alias:    *alias,
			Conn:     conn,
			LastSeen: time.Now(),
		})

		log.Printf("New client registered: %s (%s)", *clientID, *alias)
	}

	// 更新客户端最后活动时间
	if client, ok := s.clients.Load(*clientID); ok {
		client.(*ClientConnection).LastSeen = time.Now()
	}

	// 验证数据有效性
	if bandwidthData.Timestamp.IsZero() {
		log.Printf("Invalid bandwidth data: timestamp is zero")
		return
	}

	// 保存带宽数据
	err = database.SaveBandwidthData(
		*clientID,
		bandwidthData.Timestamp,
		bandwidthData.UploadSpeed,
		bandwidthData.DownloadSpeed,
		bandwidthData.AvgUploadPacketSize,
		bandwidthData.AvgDownloadPacketSize)
	if err != nil {
		log.Printf("Failed to save bandwidth data: %v", err)
		return
	}

	log.Printf("Successfully saved bandwidth data from client %s (%s)", *clientID, *alias)
}

// checkClientsStatus 定期检查客户端状态
func (s *QuicServer) checkClientsStatus() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			s.clients.Range(func(key, value interface{}) bool {
				client := value.(*ClientConnection)
				// 如果客户端超过5分钟没有活动，关闭连接
				if now.Sub(client.LastSeen) > 5*time.Minute {
					log.Printf("Client %s (%s) inactive for too long, closing connection", client.ID, client.Alias)
					client.Conn.CloseWithError(0, "inactive timeout")
					s.clients.Delete(key)
				}
				return true
			})
		}
	}
}

// GetConnectedClients 获取当前连接的客户端列表
func (s *QuicServer) GetConnectedClients() []*ClientConnection {
	var clients []*ClientConnection
	s.clients.Range(func(_, value interface{}) bool {
		if client, ok := value.(*ClientConnection); ok {
			clients = append(clients, client)
		}
		return true
	})
	return clients
}
