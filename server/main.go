package main

import (
	"network-monitor-server/router"
)

func main() {
	router := router.NewRouter()
	router.Run(":8080")
}
