package database

import (
	"database/sql"
	"fmt"
	"log"
	"network-monitor-server/config"
	
	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() {
	db, err := sql.Open("postgres", fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.Config.Database.Host, config.Config.Database.Port, config.Config.Database.User, config.Config.Database.Password, config.Config.Database.Name))
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}

	log.Println("数据库连接成功")
	DB = db
}
