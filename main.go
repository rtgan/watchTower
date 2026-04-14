package main

import (
	"fmt"
	"log"
	"watchTower/common/config"
	"watchTower/router"
)

func main() {
	conf, err := config.InitConfig()
	if err != nil {
		log.Fatalf("init config: %v", err)
	}

	host := conf.Server.Host
	port := conf.Server.Port

	//启动HTTP服务
	if err := StartServer(host, port); err != nil {
		log.Fatalf("start server: %v", err)
	}
}

func StartServer(host string, port int) error {
	r := router.InitRouter()
	return r.Run(fmt.Sprintf("%s:%d", host, port))
}
