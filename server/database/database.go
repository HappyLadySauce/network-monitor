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
	poolConfig.MaxConns = 10                       // 最大连接数
	poolConfig.MinConns = 2                        // 最小连接数
	poolConfig.MaxConnLifetime = 1 * time.Hour     // 连接最大生命周期
	poolConfig.MaxConnIdleTime = 30 * time.Minute  // 空闲连接超时时间
	poolConfig.HealthCheckPeriod = 1 * time.Minute // 健康检查周期

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
}

// initTables 初始化数据库表
func initTables() error {
	// 创建带宽统计表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS bandwidth_stats (
		id SERIAL PRIMARY KEY,
		timestamp TIMESTAMP NOT NULL,
		upload_speed DOUBLE PRECISION NOT NULL,
		download_speed DOUBLE PRECISION NOT NULL,
		avg_upload_packet_size DOUBLE PRECISION NOT NULL,
		avg_download_packet_size DOUBLE PRECISION NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- 创建时间索引以优化查询性能
	CREATE INDEX IF NOT EXISTS idx_bandwidth_stats_timestamp ON bandwidth_stats(timestamp);
	`

	_, err := Pool.Exec(ctx, createTableSQL)
	if err != nil {
		return fmt.Errorf("创建数据表失败: %v", err)
	}

	return nil
}

// Close 关闭数据库连接池
func Close() {
	if Pool != nil {
		Pool.Close()
		log.Println("数据库连接池已关闭")
	}
}

// SaveBandwidthData 保存带宽数据
func SaveBandwidthData(timestamp time.Time, uploadSpeed, downloadSpeed, avgUploadPacketSize, avgDownloadPacketSize float64) error {
	sql := `
		INSERT INTO bandwidth_stats (
			timestamp, upload_speed, download_speed, 
			avg_upload_packet_size, avg_download_packet_size
		) VALUES ($1, $2, $3, $4, $5)
	`

	_, err := Pool.Exec(ctx, sql,
		timestamp,
		uploadSpeed,
		downloadSpeed,
		avgUploadPacketSize,
		avgDownloadPacketSize)

	if err != nil {
		return fmt.Errorf("保存带宽数据失败: %v", err)
	}

	return nil
}

// GetBandwidthStats 获取指定时间范围内的带宽统计数据
func GetBandwidthStats(startTime, endTime time.Time) ([]BandwidthStat, error) {
	sql := `
		SELECT timestamp, upload_speed, download_speed, 
			   avg_upload_packet_size, avg_download_packet_size
		FROM bandwidth_stats
		WHERE timestamp BETWEEN $1 AND $2
		ORDER BY timestamp DESC
	`

	rows, err := Pool.Query(ctx, sql, startTime, endTime)
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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历带宽数据失败: %v", err)
	}

	return stats, nil
}

// BandwidthStat 带宽统计数据结构
type BandwidthStat struct {
	Timestamp             time.Time `json:"timestamp"`
	UploadSpeed           float64   `json:"upload_speed"`
	DownloadSpeed         float64   `json:"download_speed"`
	AvgUploadPacketSize   float64   `json:"avg_upload_packet_size"`
	AvgDownloadPacketSize float64   `json:"avg_download_packet_size"`
}
