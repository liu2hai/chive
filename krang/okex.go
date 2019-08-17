package krang

import (
	"chive/kfc"
	"chive/logs"
	"chive/protocol"

	"github.com/golang/protobuf/proto"
)

/*
 okex的合约面值是固定的，请参考
 https://www.okex.com/future/querylevelRate.do
 btc合约是100USD，ltc/etc/eth/bch等是10USD
*/

type okexTrade struct {
	exchange string
	uam      map[string]float32 // 合约面值
}

func NewOkexTrade() ExchangeTrade {
	return &okexTrade{
		exchange: "okex",
		uam: map[string]float32{
			"btc_usd": 100,
			"ltc_usd": 10,
			"eth_usd": 10,
			"etc_usd": 10,
			"bch_usd": 10,
		},
	}
}

func (t *okexTrade) packAndSend(tid uint32, pb proto.Message, tag string) uint32 {
	bin, err := proto.Marshal(pb)
	if err != nil {
		logs.Error("okex %s pb marshal error:%s ", tag, err.Error())
		return 0
	}

	p := &protocol.FixPackage{}
	p.Tid = tid
	p.ReqSerial = uint32(incReqSeed())
	p.Attribute = 0
	p.Payload = bin

	sbin := p.SerialToArray()
	kfc.SendMessage(protocol.TOPIC_OKEX_ARCHER_REQ, t.exchange, sbin)
	return p.ReqSerial
}

// 交易所支持得品种
func (t *okexTrade) Symbols() []string {
	return []string{"ltc_usd", "btc_usd", "etc_usd", "eth_usd", "bch_usd"}
}

// 交易所支持得合约类型
func (t *okexTrade) ContractTypes() []string {
	return []string{"this_week", "next_week", "quarter", "index"}
}

// 查询资金账户
func (t *okexTrade) QueryAccount() {
	pb := &protocol.PBFReqQryMoneyInfo{}
	pb.Exchange = []byte(t.exchange)

	t.packAndSend(protocol.FID_ReqQryMoneyInfo, pb, "account")
}

// 查询头寸
func (t *okexTrade) QueryPos(symbol string, contractType string) {
	pb := &protocol.PBFReqQryPosInfo{}
	pb.Exchange = []byte(t.exchange)
	pb.Symbol = []byte(symbol)
	pb.ContractType = []byte(contractType)

	t.packAndSend(protocol.FID_ReqQryPosInfo, pb, "pos")
}

// 下单
func (t *okexTrade) SetOrder(cmd SetOrderCmd) {
	pb := &protocol.PBFReqSetOrder{}
	pb.Exchange = []byte(cmd.Exchange)
	pb.Symbol = []byte(cmd.Symbol)
	pb.ContractType = []byte(cmd.ContractType)
	pb.Price = proto.Float32(cmd.Price)
	pb.Amount = proto.Int32(cmd.Amount)
	pb.OrderType = proto.Int32(cmd.OrderType)
	pb.PriceSt = proto.Int32(cmd.PriceSt)
	pb.Level = proto.Int32(cmd.Level)
	pb.Vol = proto.Float32(cmd.Vol)

	reqSerial := t.packAndSend(protocol.FID_ReqSetOrder, pb, "setorder")
	if reqSerial > 0 {
		kr.keeper.GetFeedBack().Add(cmd.Stname, reqSerial, protocol.FID_ReqSetOrder, "")
	}
}

// 查询单据
func (t *okexTrade) QueryOrder(symbol string, contractType string, orderId string) {
	pb := &protocol.PBFReqQryOrders{}
	pb.Exchange = []byte(t.exchange)
	pb.Symbol = []byte(symbol)
	pb.ContractType = []byte(contractType)
	pb.OrderId = []byte(orderId)

	t.packAndSend(protocol.FID_ReqQryOrders, pb, "query order by id")
}

func (t *okexTrade) QueryOrderByStatus(symbol string, contractType string, status int32) {
	pb := &protocol.PBFReqQryOrders{}
	pb.Symbol = []byte(symbol)
	pb.ContractType = []byte(contractType)
	pb.OrderId = []byte("-1")
	pb.OrderStatus = proto.Int32(status)

	t.packAndSend(protocol.FID_ReqQryOrders, pb, "query order by status")
}

// 撤销单据
func (t *okexTrade) CancelOrder(cmd SetOrderCmd) {
	pb := &protocol.PBFReqCancelOrders{}
	pb.Exchange = []byte(t.exchange)
	pb.Symbol = []byte(cmd.Symbol)
	pb.ContractType = []byte(cmd.ContractType)
	pb.OrderId = []byte(cmd.OrderIDs)

	reqSerial := t.packAndSend(protocol.FID_ReqCancelOrders, pb, "cancel order")
	if reqSerial > 0 {
		kr.keeper.GetFeedBack().Add(cmd.Stname, reqSerial, protocol.FID_ReqCancelOrders, "")
	}
}

// 合约和现货账户转账
func (t *okexTrade) TransferMoney(symbol string, transType int32, vol float32) {
	pb := &protocol.PBFReqTransferMoney{}
	pb.Exchange = []byte(t.exchange)
	pb.Symbol = []byte(symbol)
	pb.TransType = proto.Int32(transType)
	pb.Amount = proto.Float32(vol)

	t.packAndSend(protocol.FID_ReqTransferMoney, pb, "transfer money")
}

/*
 计算合约头寸的浮动盈亏
 买入：合约未实现盈亏 = (合约价值 / 结算基准价 – 合约价值 / 最新成交价) * 持仓量
 卖出：合约未实现盈亏 = ( 合约价值 / 最新成交价 - 合约价值 / 结算基准价) * 持仓量
 盈亏比：盈亏/固定保证金
*/
func (t *okexTrade) computePosProfit(pos *Pos, pb *protocol.PBFutureTick) {
	pos.LongFloatProfit = 0
	pos.LongFloatPRate = 0
	pos.ShortFloatProfit = 0
	pos.ShortFloatPRate = 0

	ua, ok := t.uam[pos.Symbol]
	if !ok {
		panic("computePosProfit can't find unit amout")
	}
	last := pb.GetLast()
	if last <= 0 {
		return
	}

	ualast := ua / last // 合约价值/最新价
	if pos.LongAmount > 0 {
		if pos.LongPriceCost <= 0 || pos.LongBond <= 0 {
			return
		}
		luacost := ua / pos.LongPriceCost
		pos.LongFloatProfit = (luacost - ualast) * pos.LongAmount
		pos.LongFloatPRate = pos.LongFloatProfit / pos.LongBond
	}

	if pos.ShortAmount > 0 {
		if pos.ShortPriceCost <= 0 || pos.ShortBond <= 0 {
			return
		}
		suacost := ua / pos.ShortPriceCost
		pos.ShortFloatProfit = (ualast - suacost) * pos.ShortAmount
		pos.ShortFloatPRate = pos.ShortFloatProfit / pos.ShortBond
	}
}

// 计算合约张数
func (t *okexTrade) ComputeContractAmount(symbol string, price float32, vol float32) int32 {
	ua, ok := t.uam[symbol]
	if !ok {
		panic("ComputeContractAmount can't find unit amout")
	}
	amount := (price * vol) / ua
	return int32(amount)
}
