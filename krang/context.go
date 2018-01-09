package krang

/*
  Context --- 是krang模块和各个strategy交互的接口
  因此开发策略只需要关注Context接口就好
  在这里重新定义SetOrderCmd, Tick, Order, Pos等结构也是为了不让pb
  这一层的协议暴露到策略这一层
*/
////////////////////////////////////////////////////////////////////////////////////////////////////

// 下单或者撤单指令
type SetOrderCmd struct {
	Stname       string // 下单的策略名称
	Exchange     string
	Symbol       string
	ContractType string
	Price        float32 // 委托价
	Amount       int32   // 合约张数，整数
	OrderType    int32   // 订单类型
	PriceSt      int32   // 价格类型
	Level        int32   // 杠杆倍数
	Vol          float32 // 币数量
	OrderIDs     string
}

// 最新行情
type Tick struct {
	Exchange     string
	Symbol       string
	ContractType string
	Timestamp    uint64

	Vol  float32 // tick内成交量
	High float32 // tick内最高价
	Low  float32 // tick内最低价

	DayVol  float32 // 24h成交量
	DayHigh float32 // 24h最高价
	DayLow  float32 // 24h最低价

	Last   float32 // 最新价
	Bid    float32 // 买一价
	Ask    float32 // 卖一价
	BidVol float32 // 买一价的量
	AskVol float32 // 卖一价的量
}

////////////////////////////////////////////////////////////////////////////////////////////////////

type Context interface {
	GetKeeper() Keeper
	GetQuoteDB() TSDB
	GetTrader(exchange string) ExchangeTrade
}

type context struct {
}

func NewContext() Context {
	return &context{}
}

func (c *context) GetKeeper() Keeper {
	return kr.keeper
}

func (c *context) GetQuoteDB() TSDB {
	return kr.quotedb
}

func (c *context) GetTrader(exchange string) ExchangeTrade {
	trader, ok := kr.traders[exchange]
	if !ok {
		return nil
	}
	return trader
}
