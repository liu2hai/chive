package main

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/liu2hai/chive/config"
	"github.com/liu2hai/chive/krang"
	"github.com/liu2hai/chive/logs"
	"github.com/liu2hai/chive/replay"
	"github.com/liu2hai/chive/utils"
)

func main() {
	utils.InitCnf()
	utils.InitLogger("replay", logs.LevelDebug)

	logs.Info("****************************************************")
	logs.Info("replay start...")
	logs.Info("appId: ", config.T.AppID)
	logs.Info("config file: ", config.T.CnfPath)
	logs.Info("replay stay debug log level")
	logs.Info("  ")
	logs.Info("  ")
	logs.Info("  ")
	logs.Info("****************************************************")

	if err := RunServer(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func RunServer() error {
	ch := make(chan int)
	r, err := replay.StartReplay(ch)
	if err != nil {
		return err
	}

	krang.SetKrangReplay(r)
	err = krang.StartKrang(ch, true)
	if err != nil {
		return err
	}

	serverLoop(ch)
	return nil
}

func serverLoop(ch chan int) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	for {
		select {
		case <-signals:
			logs.Info("recv a break signal, exit replay ...")
			<-time.After(3 * time.Second)
			return

		case <-ch:
			logs.Info("replay is all done. ")
			return
		}
	}
}
