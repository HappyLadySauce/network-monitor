package main

import (
	"network-monitor-server/config"
	"network-monitor-server/database"
	"network-monitor-server/router"
)

func main() {
	// 初始化配置
	config.InitConfig()
	// 初始化数据库
	database.InitDB()
	// 初始化路由
	router := router.InitRouter()
	router.Run(":" + config.Config.Server.Port)
}
