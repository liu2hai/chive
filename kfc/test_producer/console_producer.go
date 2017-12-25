package main

import (
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/liu2hai/chive/kfc"
	"github.com/liu2hai/chive/protocol"
)

func main() {
	brokers := []string{"localhost:9092"}
	topic := "kuche"

	kfc.InitClient(brokers)
	err := kfc.TobeProducer()
	if err != nil {
		fmt.Println("tobe error: ", err.Error())
		return
	}

	pb := &protocol.PBFReqQryMoneyInfo{}
	pb.Exchange = []byte("okex new")
	bin, err := proto.Marshal(pb)
	if err != nil {
		fmt.Println("pb marshal error: ", err.Error())
		return
	}

	p := protocol.FixPackage{}
	p.Tid = protocol.CMD_QRY_ACCOUNT
	for i := 0; i < 10; i++ {
		p.ReqSerial = uint32(i + 1)
		p.Attribute = 0
		p.Payload = bin

		sbin := p.SerialToArray()
		kfc.SendMessage(topic, "youth", sbin)
	}

	// 由于kfc send时是写缓冲的chan，所以有可能没有写完,这里等等
	<-time.After(time.Second)
	kfc.ExitProducer()
}
