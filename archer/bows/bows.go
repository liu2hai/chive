package bows

import (
	"os"
	"os/signal"

	"github.com/Shopify/sarama"
	"github.com/golang/protobuf/proto"

	"chive/kfc"
	"chive/logs"
	"chive/protocol"
)

type bowLoop struct {
	m         map[string]chan *ArcherCmd
	exchanges []string
}

func InitKafkaClient(broker string) error {
	brokers := []string{broker}
	topics := []string{protocol.TOPIC_OKEX_ARCHER_REQ}

	kfc.InitClient(brokers)
	err := kfc.TobeProducer()
	if err != nil {
		logs.Error("InitKafkaClient producer error ", err.Error())
		return err
	}
	err2 := kfc.TobeConsumer(topics)
	if err2 != nil {
		logs.Error("InitKafkaClient consumer error ", err2.Error())
		return err2
	}
	logs.Info("connect to kafka broker [%s] ok ...", broker)
	return nil
}

func InitBows() *bowLoop {
	return &bowLoop{
		m: make(map[string]chan *ArcherCmd),
	}
}

/*
 启动各个交易所下单协程
*/
func StartExArcher(exchanges []string, bl *bowLoop) error {
	bl.exchanges = exchanges
	for _, ex := range exchanges {
		q := createArchers(ex)
		if q == nil {
			logs.Error("exchange [%s] is not supported !", ex)
			continue
		}

		if err := q.Init(); err != nil {
			logs.Error("exchange [%s] init fail, error:", ex, err.Error())
			return err
		}

		ch := make(chan *ArcherCmd)
		bl.m[ex] = ch
		go q.Run(ch)
		logs.Info("start exchange [%s] archer ok ...", ex)
	}
	return nil
}

func StartCmdLoop(bl *bowLoop) {
	logs.Info("wait for cmds .....")

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	for {
		select {
		case msg := <-kfc.ReadMessages():
			handleBrokerCmd(bl, msg)

		case <-signals:
			logs.Info("recv a break signal, exit archer...")
			doExit(bl)
			return
		}
	}
}

/*
producer在发布消息时都要带上key, key就是这个exchange的名称，消费者根据这个
发给不同的exchange 处理
*/
func handleBrokerCmd(bl *bowLoop, msg *sarama.ConsumerMessage) bool {
	key := string(msg.Key)
	exch, ok := bl.m[key]
	if !ok {
		logs.Error("recv msg with wrong key [%s]", key)
		return false
	}

	p := &protocol.FixPackage{}
	if !p.ParseFromArray(msg.Value) {
		logs.Error("recv msg parse error, topic[%s]", msg.Topic)
		return false
	}

	cmd := &ArcherCmd{}
	cmd.Exchange = key
	switch p.GetTid() {
	case protocol.FID_ReqQryMoneyInfo:
		return cmdQryAccount(exch, p, cmd, msg)

	case protocol.FID_ReqQryPosInfo:
		return cmdQryPosition(exch, p, cmd, msg)

	case protocol.FID_ReqSetOrder:
		return cmdSetOrder(exch, p, cmd, msg)

	case protocol.FID_ReqQryOrders:
		return cmdQryOrders(exch, p, cmd, msg)

	case protocol.FID_ReqCancelOrders:
		return cmdCancelOrder(exch, p, cmd, msg)

	case protocol.FID_ReqTransferMoney:
		return cmdTransferMoney(exch, p, cmd, msg)

	default:
		logs.Error("recv msg not support cmd, topic[%s]", msg.Topic)
		return false
	}
	return true
}

func cmdQryAccount(exch chan<- *ArcherCmd, p protocol.Package, cmd *ArcherCmd, msg *sarama.ConsumerMessage) bool {
	pb := &protocol.PBFReqQryMoneyInfo{}
	err := proto.Unmarshal(p.GetPayload(), pb)
	if err != nil {
		logs.Error("recv msg Unmarshal error, topic[%s]", msg.Topic)
		return false
	}
	cmd.Cmd = protocol.CMD_QRY_ACCOUNT
	cmd.ReqSerial = int(p.GetReqSerial())
	cmd.Exchange = string(pb.Exchange)
	exch <- cmd
	return true
}

func cmdQryPosition(exch chan<- *ArcherCmd, p protocol.Package, cmd *ArcherCmd, msg *sarama.ConsumerMessage) bool {
	pb := &protocol.PBFReqQryPosInfo{}
	err := proto.Unmarshal(p.GetPayload(), pb)
	if err != nil {
		logs.Error("recv msg Unmarshal error, topic[%s]", msg.Topic)
		return false
	}

	cmd.Cmd = protocol.CMD_QRY_POSITION
	cmd.ReqSerial = int(p.GetReqSerial())
	cmd.Symbol = string(pb.GetSymbol())
	cmd.ContractType = string(pb.GetContractType())
	exch <- cmd
	return true
}

func cmdSetOrder(exch chan<- *ArcherCmd, p protocol.Package, cmd *ArcherCmd, msg *sarama.ConsumerMessage) bool {
	pb := &protocol.PBFReqSetOrder{}
	err := proto.Unmarshal(p.GetPayload(), pb)
	if err != nil {
		logs.Error("recv msg Unmarshal error, topic[%s]", msg.Topic)
		return false
	}

	cmd.Cmd = protocol.CMD_SET_ORDER
	cmd.ReqSerial = int(p.GetReqSerial())
	cmd.Symbol = string(pb.GetSymbol())
	cmd.ContractType = string(pb.GetContractType())
	cmd.Price = pb.GetPrice()
	cmd.Amount = int(pb.GetAmount())
	cmd.OrderType = int(pb.GetOrderType())
	cmd.PriceSt = int(pb.GetPriceSt())
	cmd.Level = int(pb.GetLevel())
	cmd.Vol = pb.GetVol()

	exch <- cmd
	return true
}

func cmdQryOrders(exch chan<- *ArcherCmd, p protocol.Package, cmd *ArcherCmd, msg *sarama.ConsumerMessage) bool {
	pb := &protocol.PBFReqQryOrders{}
	err := proto.Unmarshal(p.GetPayload(), pb)
	if err != nil {
		logs.Error("recv msg Unmarshal error, topic[%s]", msg.Topic)
		return false
	}

	cmd.Cmd = protocol.CMD_QRY_ORDERS
	cmd.ReqSerial = int(p.GetReqSerial())
	cmd.Symbol = string(pb.GetSymbol())
	cmd.ContractType = string(pb.GetContractType())
	cmd.OrderIDs = string(pb.GetOrderId())
	cmd.OrderStatus = int(pb.GetOrderStatus())
	cmd.CurrentPage = int(pb.GetCurrentPage())
	cmd.PageLength = int(pb.GetPageLength())

	exch <- cmd
	return true
}

func cmdCancelOrder(exch chan<- *ArcherCmd, p protocol.Package, cmd *ArcherCmd, msg *sarama.ConsumerMessage) bool {
	pb := &protocol.PBFReqCancelOrders{}
	err := proto.Unmarshal(p.GetPayload(), pb)
	if err != nil {
		logs.Error("recv msg Unmarshal error, topic[%s]", msg.Topic)
		return false
	}

	cmd.Cmd = protocol.CMD_CANCEL_ORDER
	cmd.ReqSerial = int(p.GetReqSerial())
	cmd.Symbol = string(pb.GetSymbol())
	cmd.ContractType = string(pb.GetContractType())
	cmd.OrderIDs = string(pb.GetOrderId())

	exch <- cmd
	return true
}

func cmdTransferMoney(exch chan<- *ArcherCmd, p protocol.Package, cmd *ArcherCmd, msg *sarama.ConsumerMessage) bool {
	pb := &protocol.PBFReqTransferMoney{}
	err := proto.Unmarshal(p.GetPayload(), pb)
	if err != nil {
		logs.Error("recv msg Unmarshal error, topic[%s]", msg.Topic)
		return false
	}

	cmd.Cmd = protocol.CMD_TRANSFER_MONEY
	cmd.ReqSerial = int(p.GetReqSerial())
	cmd.Symbol = string(pb.GetSymbol())
	cmd.TransType = int(pb.GetTransType())
	cmd.Vol = pb.GetAmount()

	exch <- cmd
	return true
}

func doExit(bl *bowLoop) {
	// exit exchanges
	cmd := &ArcherCmd{
		Cmd: INTERNAL_CMD_EXIT,
	}
	for _, v := range bl.m {
		v <- cmd
	}

	// exit kfc
	kfc.ExitProducer()
	kfc.ExitConsumer()
}

func createArchers(ex string) Archer {
	if ex == "okex" {
		return newOkexArcher()
	} else if ex == "bitfinex" {
		return newBitfinexArcher()
	}
	return nil
}
