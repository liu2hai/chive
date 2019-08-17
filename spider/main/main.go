package main

import (
	"fmt"
	"os"

	"chive/config"
	"chive/logs"
	"chive/utils"
)

func main() {
	utils.InitCnf()
	utils.InitLogger("spider", logs.LevelInfo)

	logs.Info("****************************************************")
	logs.Info("spider start...")
	logs.Info("appId: ", config.T.AppID)
	logs.Info("config file: ", config.T.CnfPath)
	logs.Info("  ")
	logs.Info("  ")
	logs.Info("  ")
	logs.Info("exchanges: %s", config.T.Exchanges)
	logs.Info("****************************************************")

	if err := RunServer(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
