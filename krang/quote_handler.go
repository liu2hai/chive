package krang

import (
	"github.com/golang/protobuf/proto"
	"github.com/liu2hai/chive/logs"
	"github.com/liu2hai/chive/protocol"
)

type quoteHandler struct {
}

func NewQuoteHandler() Handler {
	return &quoteHandler{}
}

/*
  返回true表示该handler已经处理完毕，无需后面的handler处理
  返回false表示需要给后面的handler处理
*/
func (t *quoteHandler) HandleMessage(p protocol.Package, key string) bool {
	tid := p.GetTid()
	switch tid {
	// 行情--分笔
	case protocol.FID_QUOTE_TICK:
		return quoteTick(p, key)

		// 行情--k线
	case protocol.FID_QUOTE_KLine:
		return quoteKLine(p, key)

		// 行情--档口
	case protocol.FID_QUOTE_Depth:
		return quoteDepth(p, key)

		// 行情--报价
	case protocol.FID_QUOTE_Trade:
		return quoteTrade(p, key)

		// 行情--指数
	case protocol.FID_QUOTE_Index:
		return quoteIndex(p, key)

	}

	return false
}

/*
  tick消息要放给后面的Handler处理,除非无法解包等错误
*/
func quoteTick(p protocol.Package, key string) bool {
	pb := &protocol.PBFutureTick{}
	err := proto.Unmarshal(p.GetPayload(), pb)
	if err != nil {
		logs.Error("pb unmarshal fail, tid:%d", p.GetTid())
		return true
	}
	kr.quotedb.StoreTick(pb)
	return false
}

func quoteKLine(p protocol.Package, key string) bool {
	pb := &protocol.PBFutureKLine{}
	err := proto.Unmarshal(p.GetPayload(), pb)
	if err != nil {
		logs.Error("pb unmarshal fail, tid:%d", p.GetTid())
		return true
	}
	kr.quotedb.StoreKLine(pb)
	return true
}

func quoteDepth(p protocol.Package, key string) bool {
	pb := &protocol.PBFutureDepth{}
	err := proto.Unmarshal(p.GetPayload(), pb)
	if err != nil {
		logs.Error("pb unmarshal fail, tid:%d", p.GetTid())
		return true
	}
	kr.quotedb.StoreDepth(pb)
	return true
}

func quoteTrade(p protocol.Package, key string) bool {
	pb := &protocol.PBFutureTrade{}
	err := proto.Unmarshal(p.GetPayload(), pb)
	if err != nil {
		logs.Error("pb unmarshal fail, tid:%d", p.GetTid())
		return true
	}

	kr.quotedb.StoreTrade(pb)
	return true
}

func quoteIndex(p protocol.Package, key string) bool {
	pb := &protocol.PBFutureIndex{}
	err := proto.Unmarshal(p.GetPayload(), pb)
	if err != nil {
		logs.Error("pb unmarshal fail, tid:%d", p.GetTid())
		return true
	}

	kr.quotedb.StoreIndex(pb)
	return true
}
