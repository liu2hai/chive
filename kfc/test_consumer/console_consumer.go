package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/golang/protobuf/proto"
	"github.com/liu2hai/chive/kfc"
	"github.com/liu2hai/chive/protocol"
)

func main() {
	brokers := []string{"localhost:9092"}
	topic := "kuche"
	topics := []string{topic}

	kfc.InitClient(brokers)
	err2 := kfc.TobeConsumer(topics)
	if err2 != nil {
		fmt.Println("tobe error: ", err2.Error())
		return
	}

	// Trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	p := &protocol.FixPackage{}
	pb := protocol.PBFReqQryMoneyInfo{}
L:
	for {
		select {
		case msg := <-kfc.ReadMessages():
			if !p.ParseFromArray(msg.Value) {
				fmt.Println("consumer parse fail")
				continue
			}

			err := proto.Unmarshal(p.GetPayload(), &pb)
			if err != nil {
				fmt.Println("unmarchal error ", err.Error())
			} else {
				fmt.Println("recv msg: ", p.GetTid(), p.GetReqSerial(), string(pb.Exchange))
			}
		case <-signals:
			fmt.Println("recv a break")
			break L
		}
	}
	kfc.ExitConsumer()
}
