package main

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/liu2hai/chive/config"
	"github.com/liu2hai/chive/kfc"
	"github.com/liu2hai/chive/krang"
	"github.com/liu2hai/chive/logs"
	"github.com/liu2hai/chive/protocol"
	"github.com/liu2hai/chive/strategy/mavg"
	"github.com/liu2hai/chive/utils"
)

func main() {
	utils.InitCnf()
	utils.InitLogger("krang", logs.LevelInfo)

	logs.Info("****************************************************")
	logs.Info("krang start...")
	logs.Info("Hello humna being, I'm krang from TMNT")
	logs.Info("  ")
	logs.Info("  ")
	logs.Info("  ")
	logs.Info("appId: ", config.T.AppID)
	logs.Info("config file: ", config.T.CnfPath)
	logs.Info("exchange: ", config.T.Exchanges)
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

	err := kfc.TobeProducer()
	if err != nil {
		logs.Error("Init Kafka producer error ", err.Error())
		return err
	}

	err = kfc.TobeConsumer(topics)
	if err != nil {
		logs.Error("Init Kafka consumer error ", err.Error())
		return err
	}
	logs.Info("connect to kafka broker [%s] ok ...", config.T.Broker)

	// 注册策略, 因为strategy是依赖krang的，所以如果在krang里注册会有依赖问题
	mavg.RegisStrategy()

	ch := make(chan int)
	err = krang.StartKrang(ch, false)
	if err != nil {
		return err
	}

	serverLoop(ch)
	return nil
}

func serverLoop(ch chan int) {
	defer close(ch)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	<-signals
	logs.Info("recv a break signal, exit krang...")
	ch <- 1
	<-time.After(3 * time.Second)
	kfc.ExitConsumer()
	kfc.ExitProducer()
}
