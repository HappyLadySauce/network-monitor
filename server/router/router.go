package router

import (
	"github.com/gin-gonic/gin"
	"network-monitor-server/middlewares"
)


func InitRouter() *gin.Engine {
	router := gin.Default()
	router.Use(middlewares.CORSMiddleware())
	return router
}