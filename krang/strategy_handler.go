package krang

import (
	"chive/logs"
	"chive/protocol"

	"github.com/golang/protobuf/proto"
)

type strategyHandler struct {
}

func NewStrategyHandler() Handler {
	return &strategyHandler{}
}

/*
  返回true表示该handler已经处理完毕，无需后面的handler处理
  返回false表示需要给后面的handler处理
*/
func (t *strategyHandler) HandleMessage(p protocol.Package, key string) bool {
	if p.GetTid() != protocol.FID_QUOTE_TICK {
		return false
	}

	pb := &protocol.PBFutureTick{}
	err := proto.Unmarshal(p.GetPayload(), pb)
	if err != nil {
		logs.Error("pb unmarshal fail, tid:%d", p.GetTid())
		return true
	}
	tick := &Tick{
		Exchange:     pb.GetSinfo().GetExchange(),
		Symbol:       pb.GetSinfo().GetSymbol(),
		ContractType: pb.GetSinfo().GetContractType(),
		Timestamp:    pb.GetSinfo().GetTimestamp(),
		Vol:          pb.GetVol(),
		High:         pb.GetHigh(),
		Low:          pb.GetLow(),
		DayVol:       pb.GetDayVol(),
		DayHigh:      pb.GetDayHigh(),
		DayLow:       pb.GetDayLow(),
		Last:         pb.GetLast(),
		Bid:          pb.GetBid(),
		Ask:          pb.GetAsk(),
		BidVol:       pb.GetBidVol(),
		AskVol:       pb.GetAskVol(),
	}

	// 策略处理
	for _, v := range kr.stmgr.m {
		if !v.CheckFeedBack(kr.ctx) {
			continue
		}
		v.OnTick(kr.ctx, tick)
	}

	return false
}
