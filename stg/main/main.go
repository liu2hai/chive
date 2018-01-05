package main

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/liu2hai/chive/config"
	"github.com/liu2hai/chive/kfc"
	"github.com/liu2hai/chive/logs"
	"github.com/liu2hai/chive/protocol"
	"github.com/liu2hai/chive/stg"
	"github.com/liu2hai/chive/utils"
)

func main() {
	utils.InitCnf()
	utils.InitLogger("stg", logs.LevelInfo)

	logs.Info("****************************************************")
	logs.Info("storage start...")
	logs.Info("appId: ", config.T.AppID)
	logs.Info("config file: ", config.T.CnfPath)
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
	brokers := []string{config.T.Broker}
	topics := []string{protocol.TOPIC_OKEX_QUOTE_PUB, protocol.TOPIC_OKEX_ARCHER_RSP}

	kfc.InitClient(brokers)
	err := kfc.TobeConsumer(topics)
	if err != nil {
		logs.Error("Init Kafka consumer error ", err.Error())
		return err
	}
	logs.Info("connect to kafka broker [%s] ok ...", config.T.Broker)

	ch := make(chan int)
	if err := stg.StartStorage(ch); err != nil {
		return err
	}

	serverLoop(ch)
	return nil
}

func serverLoop(ch chan int) {
	defer close(ch)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	tc := time.NewTimer(time.Second)
	defer tc.Stop()

	for {
		select {
		case <-signals:
			logs.Info("recv a break signal, exit storage...")
			ch <- stg.STG_CMD_EXIT
			<-time.After(time.Second)
			kfc.ExitConsumer()
			return

		case <-tc.C:
			tc.Reset(time.Second)
			if isEndOfDay() {
				ch <- stg.STG_CMD_SWITCH_TRADINGDAY
			}
		}
	}
}

func isEndOfDay() bool {
	t := time.Now()
	if t.Hour() == 0 && t.Minute() == 0 && (t.Second() >= 0 || t.Second() < 2) {
		return true
	}
	return false
}
