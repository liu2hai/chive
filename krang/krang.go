/*
 krang --- 忍者神龟里的大脑人朗格，负责下单和决策

 1. krang是一个单协程程序，所有策略都由它来执行
 2. krang使用内存数据库tsdb保存行情，报价行情驱动策略
*/

package krang

import (
	"errors"
	"sync/atomic"

	"chive/config"
	"chive/kfc"
	"chive/logs"
	"chive/protocol"
	"chive/replay"

	"github.com/Shopify/sarama"
)

type Handler interface {
	HandleMessage(protocol.Package, string) bool
}

/*
  ExchangeTrade 交易所交易所需要的接口
*/
type ExchangeTrade interface {

	// 交易所支持得品种
	Symbols() []string

	// 交易所支持得合约类型
	ContractTypes() []string

	// 查询资金账户
	QueryAccount()

	// 查询头寸
	QueryPos(symbol string, contractType string)

	// 下单
	SetOrder(cmd SetOrderCmd)

	// 查询单据
	QueryOrder(symbol string, contractType string, orderId string)
	QueryOrderByStatus(symbol string, contractType string, status int32)

	// 撤销单据
	CancelOrder(cmd SetOrderCmd)

	// 合约和现货账户转账
	TransferMoney(symbol string, transType int32, vol float32)

	// 计算合约张数
	ComputeContractAmount(symbol string, price float32, vol float32) int32

	// 计算持仓盈亏，每个交易所计算方法不一样
	// 这个方法不会暴露给策略使用
	computePosProfit(pos *Pos, pb *protocol.PBFutureTick)
}

type krang struct {
	traders  map[string]ExchangeTrade
	keeper   Keeper
	handlers []Handler
	quotedb  TSDB
	stmgr    *StrategyManager
	ctx      Context
	exitCh   chan int
	reqSeed  int64
	replay   *replay.Replay
}

var kr *krang

////////////////////////////////////////////////////////

/*
 StartKrang --- 初始化工作和启动krang协程
*/
func StartKrang(exitCh chan int, bReplay bool) error {
	kr.exitCh = exitCh
	kr.keeper = NewKeeper()

	// 启动tsdb
	kr.quotedb = NewTSDBClient(bReplay)
	for _, v := range config.T.Exchanges {
		t, err := createTrader(v)
		if err != nil {
			return err
		}
		kr.traders[v] = t
		kr.quotedb.Check(v, t.Symbols(), t.ContractTypes())
	}
	kr.quotedb.Start()

	// 消息处理handlers,处理顺序为：trade -> quote -> strategy
	h := NewTradeHandler()
	kr.handlers = append(kr.handlers, h)
	h1 := NewQuoteHandler()
	kr.handlers = append(kr.handlers, h1)
	h2 := NewStrategyHandler()
	kr.handlers = append(kr.handlers, h2)

	// 获得注册的策略
	kr.stmgr = GetStrategyMgr()

	// 初始化context
	kr.ctx = NewContext()

	go krangLoop(bReplay)
	return nil
}

/*
 如果需要回放，需要设置回放模式
*/
func SetKrangReplay(r *replay.Replay) {
	if r == nil {
		panic("SetKrangReplay param vaild")
	}
	kr.replay = r
}

////////////////////////////////////////////////////////
func krangLoop(bReplay bool) {
	defer krangExit()

	// 初始化策略
	for _, v := range kr.stmgr.m {
		v.Init(kr.ctx)
	}

	var pumpCh <-chan *sarama.ConsumerMessage
	if bReplay {
		pumpCh = kr.replay.ReadMessages()
	} else {
		pumpCh = kfc.ReadMessages()
	}

	for {
		select {
		case msg := <-pumpCh:
			handlemsg(msg)

		case <-kr.exitCh:
			return
		}
	}
}

func handlemsg(msg *sarama.ConsumerMessage) bool {
	p := &protocol.FixPackage{}
	if !p.ParseFromArray(msg.Value) {
		logs.Error("krang consumer msg parse fail")
		return false
	}

	key := string(msg.Key)
	for _, h := range kr.handlers {
		b := h.HandleMessage(p, key)
		if b {
			return true
		}
	}
	return true
}

func krangExit() {
	kr.quotedb.Close()
}

////////////////////////////////////////////////////////

func createTrader(exchange string) (ExchangeTrade, error) {
	if exchange == "okex" {
		return NewOkexTrade(), nil
	}
	return nil, errors.New("create exchange trader, not supported exchange")
}

func incReqSeed() int64 {
	atomic.AddInt64(&kr.reqSeed, 1)
	return kr.reqSeed
}

func init() {
	kr = &krang{
		traders:  make(map[string]ExchangeTrade),
		handlers: make([]Handler, 0),
		reqSeed:  0,
		replay:   nil,
	}
}
