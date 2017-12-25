package main

import (
	"os"
	"os/signal"
	"time"

	"github.com/liu2hai/chive/config"
	"github.com/liu2hai/chive/kfc"
	"github.com/liu2hai/chive/logs"
	"github.com/liu2hai/chive/spider/front"
)

// RunServer  start servers
func RunServer() error {
	brokers := []string{config.T.Broker}
	exchanges := config.T.Exchanges

	kfc.InitClient(brokers)
	err := kfc.TobeProducer()
	if err != nil {
		logs.Error("InitKafkaClient producer error ", err.Error())
		return err
	}
	logs.Info("connect to kafka broker [%s] ok ...", config.T.Broker)

	if err := front.StartQuoters(exchanges); err != nil {
		return err
	}

	serverLoop()
	return nil
}

func serverLoop() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	<-signals
	logs.Info("recv a break signal, exit spider...")
	kfc.ExitProducer()

	<-time.After(time.Second)
}
