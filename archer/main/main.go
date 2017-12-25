package main

import (
	"fmt"
	"os"

	"github.com/liu2hai/chive/config"
	"github.com/liu2hai/chive/logs"
	"github.com/liu2hai/chive/utils"
)

func main() {
	utils.InitCnf()
	utils.InitLogger("archer", logs.LevelInfo)

	logs.Info("****************************************************")
	logs.Info("archer start...")
	logs.Info("appId: ", config.T.AppID)
	logs.Info("config file: ", config.T.CnfPath)
	logs.Info("  ")
	logs.Info("  ")
	logs.Info("  ")
	logs.Info("exchanges: ", config.T.Exchanges)
	logs.Info("broker: ", config.T.Broker)
	logs.Info("****************************************************")

	if err := RunServer(); err != nil {
		fmt.Println(err)
		logs.Error("archer exit -1")
		os.Exit(-1)
	}
}
