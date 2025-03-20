package database

import (
	"context"
	"fmt"
	"log"
	"network-monitor-server/config"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	Pool *pgxpool.Pool
	ctx  context.Context
)

// InitDB 初始化数据库连接池
func InitDB() {
	var err error
	ctx = context.Background()

	// 构建连接字符串
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		config.Config.Database.User,
		config.Config.Database.Password,
		config.Config.Database.Host,
		config.Config.Database.Port,
		config.Config.Database.Name)

	// 配置连接池
	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		log.Fatalf("解析数据库配置失败: %v", err)
	}

	// 设置连接池参数
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = 1 * time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	// 创建连接池
	Pool, err = pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatalf("创建数据库连接池失败: %v", err)
	}

	// 测试连接
	if err := Pool.Ping(ctx); err != nil {
		log.Fatalf("数据库连接测试失败: %v", err)
	}

	log.Println("数据库连接池初始化成功")

	// 初始化数据库表
	if err := initTables(); err != nil {
		log.Fatalf("初始化数据库表失败: %v", err)
	}

	// 启动定期清理任务
	go startCleanupTask()
}

// initTables 初始化数据库表
func initTables() error {
	// 创建客户端表
	createClientTableSQL := `
	CREATE TABLE IF NOT EXISTS clients (
		id SERIAL PRIMARY KEY,
		client_id VARCHAR(64) UNIQUE NOT NULL,
		alias VARCHAR(128) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		last_seen_at TIMESTAMP
	);

	-- 创建客户端ID索引
	CREATE INDEX IF NOT EXISTS idx_clients_client_id ON clients(client_id);
	`

	// 创建带宽统计表
	createBandwidthTableSQL := `
	CREATE TABLE IF NOT EXISTS bandwidth_stats (
		id SERIAL PRIMARY KEY,
		client_id VARCHAR(64) NOT NULL,
		timestamp TIMESTAMP NOT NULL,
		upload_speed DOUBLE PRECISION NOT NULL,
		download_speed DOUBLE PRECISION NOT NULL,
		avg_upload_packet_size DOUBLE PRECISION NOT NULL,
		avg_download_packet_size DOUBLE PRECISION NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (client_id) REFERENCES clients(client_id) ON DELETE CASCADE
	);

	-- 创建时间和客户端ID的复合索引
	CREATE INDEX IF NOT EXISTS idx_bandwidth_stats_client_timestamp 
	ON bandwidth_stats(client_id, timestamp DESC);
	`

	// 执行创建表SQL
	if _, err := Pool.Exec(ctx, createClientTableSQL); err != nil {
		return fmt.Errorf("创建客户端表失败: %v", err)
	}

	if _, err := Pool.Exec(ctx, createBandwidthTableSQL); err != nil {
		return fmt.Errorf("创建带宽统计表失败: %v", err)
	}

	return nil
}

// RegisterClient 注册或更新客户端信息
func RegisterClient(clientID, alias string) error {
	sql := `
		INSERT INTO clients (client_id, alias, last_seen_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (client_id) 
		DO UPDATE SET 
			alias = EXCLUDED.alias,
			last_seen_at = CURRENT_TIMESTAMP
	`

	_, err := Pool.Exec(ctx, sql, clientID, alias)
	if err != nil {
		return fmt.Errorf("注册客户端失败: %v", err)
	}

	return nil
}

// UpdateClientLastSeen 更新客户端最后在线时间
func UpdateClientLastSeen(clientID string) error {
	sql := `
		UPDATE clients 
		SET last_seen_at = CURRENT_TIMESTAMP
		WHERE client_id = $1
	`

	_, err := Pool.Exec(ctx, sql, clientID)
	if err != nil {
		return fmt.Errorf("更新客户端最后在线时间失败: %v", err)
	}

	return nil
}

// SaveBandwidthData 保存带宽数据
func SaveBandwidthData(clientID string, timestamp time.Time, uploadSpeed, downloadSpeed, avgUploadPacketSize, avgDownloadPacketSize float64) error {
	sql := `
		INSERT INTO bandwidth_stats (
			client_id, timestamp, upload_speed, download_speed, 
			avg_upload_packet_size, avg_download_packet_size
		) VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := Pool.Exec(ctx, sql,
		clientID,
		timestamp,
		uploadSpeed,
		downloadSpeed,
		avgUploadPacketSize,
		avgDownloadPacketSize)

	if err != nil {
		return fmt.Errorf("保存带宽数据失败: %v", err)
	}

	// 更新客户端最后在线时间
	return UpdateClientLastSeen(clientID)
}

// GetBandwidthStats 获取指定客户端的带宽统计数据
func GetBandwidthStats(clientID string, startTime, endTime time.Time) ([]BandwidthStat, error) {
	sql := `
		SELECT timestamp, upload_speed, download_speed, 
			   avg_upload_packet_size, avg_download_packet_size
		FROM bandwidth_stats
		WHERE client_id = $1 AND timestamp BETWEEN $2 AND $3
		ORDER BY timestamp DESC
	`

	rows, err := Pool.Query(ctx, sql, clientID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("查询带宽数据失败: %v", err)
	}
	defer rows.Close()

	var stats []BandwidthStat
	for rows.Next() {
		var stat BandwidthStat
		err := rows.Scan(
			&stat.Timestamp,
			&stat.UploadSpeed,
			&stat.DownloadSpeed,
			&stat.AvgUploadPacketSize,
			&stat.AvgDownloadPacketSize,
		)
		if err != nil {
			return nil, fmt.Errorf("解析带宽数据失败: %v", err)
		}
		stats = append(stats, stat)
	}

	return stats, nil
}

// GetClientInfo 获取客户端信息
func GetClientInfo(clientID string) (*ClientInfo, error) {
	sql := `
		SELECT client_id, alias, created_at, last_seen_at
		FROM clients
		WHERE client_id = $1
	`

	var client ClientInfo
	err := Pool.QueryRow(ctx, sql, clientID).Scan(
		&client.ClientID,
		&client.Alias,
		&client.CreatedAt,
		&client.LastSeenAt,
	)

	if err != nil {
		return nil, fmt.Errorf("获取客户端信息失败: %v", err)
	}

	return &client, nil
}

// startCleanupTask 启动定期清理任务
func startCleanupTask() {
	ticker := time.NewTicker(24 * time.Hour) // 每天执行一次清理
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := cleanupOldData(); err != nil {
				log.Printf("清理旧数据失败: %v", err)
			}
		}
	}
}

// cleanupOldData 清理超过一周的数据
func cleanupOldData() error {
	sql := `
		DELETE FROM bandwidth_stats
		WHERE timestamp < NOW() - INTERVAL '7 days'
	`

	_, err := Pool.Exec(ctx, sql)
	if err != nil {
		return fmt.Errorf("清理旧数据失败: %v", err)
	}

	log.Println("已清理超过一周的数据")
	return nil
}

// Close 关闭数据库连接池
func Close() {
	if Pool != nil {
		Pool.Close()
		log.Println("数据库连接池已关闭")
	}
}

// BandwidthStat 带宽统计数据结构
type BandwidthStat struct {
	Timestamp             time.Time `json:"timestamp"`
	UploadSpeed           float64   `json:"upload_speed"`
	DownloadSpeed         float64   `json:"download_speed"`
	AvgUploadPacketSize   float64   `json:"avg_upload_packet_size"`
	AvgDownloadPacketSize float64   `json:"avg_download_packet_size"`
}

// ClientInfo 客户端信息结构
type ClientInfo struct {
	ClientID   string    `json:"client_id"`
	Alias      string    `json:"alias"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}
