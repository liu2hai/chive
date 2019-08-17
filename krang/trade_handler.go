package krang

import (
	"chive/logs"
	"chive/protocol"

	"github.com/golang/protobuf/proto"
)

type tradeHandler struct {
}

func NewTradeHandler() Handler {
	return &tradeHandler{}
}

/*
  返回true表示该handler已经处理完毕，无需后面的handler处理
  返回false表示需要给后面的handler处理
*/
func (t *tradeHandler) HandleMessage(p protocol.Package, key string) bool {
	tid := p.GetTid()
	switch tid {

	// 响应最新行情
	case protocol.FID_QUOTE_TICK:
		return onTick(p, key)

	// 查询资金信息回应
	case protocol.FID_RspQryMoneyInfo:
		return rspQryMoneyInfo(p, key)

	// 查询头寸回应
	case protocol.FID_RspQryPosInfo:
		return rspQryPosInfo(p, key)

		// 下单回应
	case protocol.FID_RspSetOrder:
		return rspSetOrder(p, key)

		// 批量查询单据回应
	case protocol.FID_RspQryOrders:
		return rspQryOrders(p, key)

		// 批量撤销单据回应
	case protocol.FID_RspCancelOrders:
		return rspCancelOrders(p, key)

		// 在现货和合约账号划转资金回应
	case protocol.FID_RspTransferMoney:
		return rspTransferMoney(p, key)
	}
	return false
}

func onTick(p protocol.Package, key string) bool {
	pb := &protocol.PBFutureTick{}
	err := proto.Unmarshal(p.GetPayload(), pb)
	if err != nil {
		logs.Error("pb unmarshal fail, tid:%d", p.GetTid())
		return true
	}
	kr.keeper.OnTick(key, pb)
	return false
}

func rspQryMoneyInfo(p protocol.Package, key string) bool {
	pb := &protocol.PBFRspQryMoneyInfo{}
	err := proto.Unmarshal(p.GetPayload(), pb)
	if err != nil {
		logs.Error("pb unmarshal fail, tid:%d", p.GetTid())
		return true
	}

	kr.keeper.HandleMoney(key, pb)
	return true
}

func rspQryPosInfo(p protocol.Package, key string) bool {
	pb := &protocol.PBFRspQryPosInfo{}
	err := proto.Unmarshal(p.GetPayload(), pb)
	if err != nil {
		logs.Error("pb unmarshal fail, tid:%d", p.GetTid())
		return true
	}

	kr.keeper.HandlePos(key, pb)
	return true
}

// 下单回应后，立即查询该订单状态和资金头寸
func rspSetOrder(p protocol.Package, key string) bool {
	pb := &protocol.PBFRspSetOrder{}
	err := proto.Unmarshal(p.GetPayload(), pb)
	if err != nil {
		logs.Error("pb unmarshal fail, tid:%d", p.GetTid())
		return true
	}

	trader, ok := kr.traders[key]
	if !ok {
		return true
	}
	s := string(pb.GetSymbol())
	c := string(pb.GetContractType())
	trader.QueryAccount()
	trader.QueryPos(s, c)

	if pb.GetRsp().GetErrorId() != protocol.ErrId_OK {
		logs.Info("下单失败，原因：%s", string(pb.GetRsp().GetErrorMsg()))
		return true
	}

	// 更新反馈信息
	kr.keeper.GetFeedBack().Remove(p.GetReqSerial())
	id := string(pb.GetOrderId())
	trader.QueryOrder(s, c, id)
	return true
}

func rspQryOrders(p protocol.Package, key string) bool {
	pb := &protocol.PBFRspQryOrders{}
	err := proto.Unmarshal(p.GetPayload(), pb)
	if err != nil {
		logs.Error("pb unmarshal fail, tid:%d", p.GetTid())
		return true
	}

	kr.keeper.HandleOrders(key, pb)
	return true
}

//撤单后， 查询撤销单据的状态
func rspCancelOrders(p protocol.Package, key string) bool {
	pb := &protocol.PBFRspCancelOrders{}
	err := proto.Unmarshal(p.GetPayload(), pb)
	if err != nil {
		logs.Error("pb unmarshal fail, tid:%d", p.GetTid())
		return true
	}
	if pb.GetRsp().GetErrorId() != protocol.ErrId_OK {
		logs.Info("撤单失败，原因：%s", string(pb.GetRsp().GetErrorMsg()))
		return true
	}

	// 先更新反馈信息
	kr.keeper.GetFeedBack().Remove(p.GetReqSerial())

	trader, ok := kr.traders[key]
	if !ok {
		return true
	}
	ids := []string{}
	for _, v := range pb.GetSuccess() {
		ids = append(ids, string(v))
	}
	for _, v := range pb.GetErrors() {
		ids = append(ids, string(v))
	}

	s := string(pb.GetSymbol())
	c := string(pb.GetContractType())
	for _, id := range ids {
		trader.QueryOrder(s, c, id)
	}

	return true
}

// 转账后，立即查询资金状态
func rspTransferMoney(p protocol.Package, key string) bool {
	pb := &protocol.PBFRspTransferMoney{}
	err := proto.Unmarshal(p.GetPayload(), pb)
	if err != nil {
		logs.Error("pb unmarshal fail, tid:%d", p.GetTid())
		return true
	}

	trader, ok := kr.traders[key]
	if !ok {
		return true
	}
	trader.QueryAccount()
	return true
}
