package krang

import (
	"container/list"

	"chive/protocol"
)

/*
 记录持仓单信息
 记录头寸信息
 记录交易历史
 记录发出去请求的状态，是否已经收到回复，还是回复错误，以及重试次数
 这些信息对策略有决定作用
*/

type Keeper interface {
	// 根据资金信息更新缓存
	HandleMoney(exchange string, pb *protocol.PBFRspQryMoneyInfo) bool

	// 根据订单信息更新缓存
	HandleOrders(exchange string, pb *protocol.PBFRspQryOrders) bool

	// 根据头寸信息更新缓存
	HandlePos(exchange string, pb *protocol.PBFRspQryPosInfo) bool

	// 响应最新报价
	OnTick(exchange string, pb *protocol.PBFutureTick)

	// 根据订单id查找订单
	GetOrderById(orderId string) *Order

	// 根据商品信息查找一个或多个订单
	GetOrderBySinfo(exchange string, symbol string, contractType string) []*Order

	// 根据商品信息查找头寸
	GetPos(exchange string, symbol string, contractType string) *Pos

	// 查找商品资金信息
	GetMoney(exchange string, symbol string) *Money

	// 查找回馈信息
	GetFeedBack() FeedBack
}

type Order struct {
	Exchange     string
	Symbol       string
	ContractType string

	Amount       float32 // 委托数量
	ContractName string  // 合约名称
	ContractDate string  // 委托时间
	DealAmount   float32 // 成交数量
	Fee          float32 // 手续费
	OrderId      string  // 订单id
	Price        float32 // 订单委托价格
	PriceAvg     float32 // 订单成交平均价格
	OrderStatus  int32   // 订单状态
	OrderType    int32   // 订单类型
	UnitAmount   float32 // 合约面值
	Lever        int32   // 杠杆倍数
	FloatProfit  float32 // 浮动盈亏
	CloseProfit  float32 // 平仓盈亏
}

type Pos struct {
	Exchange     string
	Symbol       string
	ContractType string
	IsValid      bool // 是否是最新的pos信息

	LongAmount      float32 // 多头合约张数
	LongAvai        float32 // 多头可平数量
	LongBond        float32 // 多头保证金
	LongFlatPrice   float32 // 多头强平价格
	LongPriceAvg    float32 // 多头开仓平均价
	LongPriceCost   float32 // 多头结算基准价
	LongCloseProfit float32 // 多头已平仓盈亏
	LongFloatProfit float32 // 多头浮动盈亏
	LongFloatPRate  float32 // 多头浮动盈亏比例

	ShortAmount      float32 // 空头合约张数
	ShortAvai        float32 // 空头可平数量
	ShortBond        float32 // 空头保证金
	ShortFlatPrice   float32 // 空头强平价格
	ShortPriceAvg    float32 // 空头开仓平均价
	ShortPriceCost   float32 // 空头结算基准价
	ShortCloseProfit float32 // 空头已平仓盈亏
	ShortFloatProfit float32 // 空头浮动盈亏
	ShortFloatPRate  float32 // 空头浮动盈亏比例
}

func (p *Pos) OnTick(exchange string, pb *protocol.PBFutureTick) {
	trader, ok := kr.traders[exchange]
	if !ok {
		return
	}

	trader.computePosProfit(p, pb)
}

func (p *Pos) Disable() {
	p.IsValid = false
}

// 清除数据部分
func (p *Pos) Reset() {
	p.IsValid = true
	p.LongAmount = 0      // 多头合约张数
	p.LongAvai = 0        // 多头可平数量
	p.LongBond = 0        // 多头保证金
	p.LongFlatPrice = 0   // 多头强平价格
	p.LongPriceAvg = 0    // 多头开仓平均价
	p.LongPriceCost = 0   // 多头结算基准价
	p.LongCloseProfit = 0 // 多头已平仓盈亏

	p.ShortAmount = 0      // 空头合约张数
	p.ShortAvai = 0        // 空头可平数量
	p.ShortBond = 0        // 空头保证金
	p.ShortFlatPrice = 0   // 空头强平价格
	p.ShortPriceAvg = 0    // 空头开仓平均价
	p.ShortPriceCost = 0   // 空头结算基准价
	p.ShortCloseProfit = 0 // 空头已平仓盈亏
}

type Money struct {
	Exchange string
	Symbol   string

	Balance float32 // 该品种的可用余额
	Rights  float32 // 该品种的权益，权益= 余额 + 市值
}

func (m *Money) OnTick(exchange string, pb *protocol.PBFutureTick) {

}

type keeper struct {
	orders   *list.List
	pos      []*Pos
	moneys   []*Money
	feedback FeedBack
}

func NewKeeper() *keeper {
	return &keeper{
		orders:   list.New(),
		pos:      make([]*Pos, 0),
		moneys:   make([]*Money, 0),
		feedback: NewFeedBack(),
	}
}

func (k *keeper) findOrder(orderId string) *list.Element {
	for e := k.orders.Front(); e != nil; e = e.Next() {
		o := e.Value.(*Order)
		if o.OrderId == orderId {
			return e
		}
	}
	return nil
}

func (k *keeper) findPos(exchange string, symbol string, contractType string) int {
	for i, v := range k.pos {
		if v.Exchange == exchange && v.Symbol == symbol && v.ContractType == contractType {
			return i
		}
	}
	return -1
}

func (k *keeper) findMoney(exchange string, symbol string) int {
	for i, v := range k.moneys {
		if v.Exchange == exchange && v.Symbol == symbol {
			return i
		}
	}
	return -1
}

func isUndoneOrder(status int32) bool {
	return status == protocol.ORDERSTATUS_WAITTING || status == protocol.ORDERSTATUS_PARTDONE
}

/*
 keeper里的order只保留没有成交或者部分成交的委托
 如果该委托成交了，则从keeper里删除
*/
func (k *keeper) HandleOrders(exchange string, pb *protocol.PBFRspQryOrders) bool {
	if pb.GetRsp().GetErrorId() != protocol.ErrId_OK {
		return false
	}

	for _, v := range pb.GetOrders() {
		id := string(v.GetOrderId())
		e := k.findOrder(id)
		if e == nil {
			if !isUndoneOrder(v.GetStatus()) {
				continue
			}
			o := &Order{}
			o.Exchange = exchange
			o.Symbol = string(v.GetSymbol())
			o.ContractType = string(v.GetContractType())
			o.Amount = v.GetAmount()
			o.ContractName = string(v.GetContractName())
			o.ContractDate = string(v.GetContractDate())
			o.DealAmount = v.GetDealAmount()
			o.Fee = v.GetFee()
			o.OrderId = id
			o.Price = v.GetPrice()
			o.PriceAvg = v.GetPriceAvg()
			o.OrderStatus = v.GetStatus()
			o.OrderType = v.GetType()
			o.UnitAmount = v.GetUnitAmount()
			o.Lever = v.GetLeverRate()
			o.FloatProfit = 0
			o.CloseProfit = 0
			k.orders.PushBack(o)
		} else {
			if !isUndoneOrder(v.GetStatus()) {
				k.orders.Remove(e)
			}
		}
	}
	return true
}

/*
   头寸信息以服务器发来的回应为准
   1. 如果这个商品返回的头寸信息为空，删除或者reset本地的头寸信息
   2. 如果这个商品返回的头寸信息不存在本地，添加
   3. 如果这个商品返回的头寸信息存在本地，更新
*/
func (k *keeper) HandlePos(exchange string, pb *protocol.PBFRspQryPosInfo) bool {
	if pb.GetRsp().GetErrorId() != protocol.ErrId_OK {
		return false
	}

	symbol := string(pb.GetSymbol())
	contractType := string(pb.GetContractType())
	idx := k.findPos(exchange, symbol, contractType)

	if len(pb.GetPosInfos()) <= 0 {
		if idx >= 0 {
			k.pos[idx].Reset()
		}
	} else {
		var pos *Pos = nil
		if idx < 0 {
			p := &Pos{}
			p.Exchange = exchange
			p.Symbol = symbol
			p.ContractType = contractType
			k.pos = append(k.pos, p)
			pos = p

		} else {
			pos = k.pos[idx]
		}

		v := pb.GetPosInfos()[0]
		pos.IsValid = true
		pos.LongAmount = v.GetBuyAmount()          // 多头合约张数
		pos.LongAvai = v.GetBuyAvailable()         // 多头可平数量
		pos.LongBond = v.GetBuyBond()              // 多头保证金
		pos.LongFlatPrice = v.GetBuyFlatprice()    // 多头强平价格
		pos.LongPriceAvg = v.GetBuyPriceAvg()      // 多头开仓平均价
		pos.LongPriceCost = v.GetBuyPriceCost()    // 多头结算基准价
		pos.LongCloseProfit = v.GetBuyProfitReal() // 多头已平仓盈亏

		pos.ShortAmount = v.GetSellAmount()          // 空头合约张数
		pos.ShortAvai = v.GetSellAvailable()         // 空头可平数量
		pos.ShortBond = v.GetSellBond()              // 空头保证金
		pos.ShortFlatPrice = v.GetSellFlatprice()    // 空头强平价格
		pos.ShortPriceAvg = v.GetSellPriceAvg()      // 空头开仓平均价
		pos.ShortPriceCost = v.GetSellPriceCost()    // 空头结算基准价
		pos.ShortCloseProfit = v.GetSellProfitReal() // 空头已平仓盈亏
	}
	return true
}

func (k *keeper) HandleMoney(exchange string, pb *protocol.PBFRspQryMoneyInfo) bool {
	if pb.GetRsp().GetErrorId() != protocol.ErrId_OK {
		return false
	}
	for _, v := range pb.GetMoneyInfos() {
		symbol := string(v.GetSymbol())
		idx := k.findMoney(exchange, symbol)
		if idx < 0 {
			m := &Money{}
			m.Exchange = exchange
			m.Symbol = symbol
			m.Balance = v.GetBalance()
			m.Rights = v.GetRights()
			k.moneys = append(k.moneys, m)

		} else {
			k.moneys[idx].Balance = v.GetBalance()
			k.moneys[idx].Rights = v.GetRights()
		}
	}
	return true
}

func (k *keeper) OnTick(exchange string, pb *protocol.PBFutureTick) {
	for _, p := range k.pos {
		p.OnTick(exchange, pb)
	}

	for _, m := range k.moneys {
		m.OnTick(exchange, pb)
	}
}

func (k *keeper) GetOrderById(orderId string) *Order {
	e := k.findOrder(orderId)
	if e != nil {
		o := e.Value.(*Order)
		return o
	}
	return nil
}

func (k *keeper) GetOrderBySinfo(exchange string, symbol string, contractType string) []*Order {
	ret := []*Order{}
	for e := k.orders.Front(); e != nil; e = e.Next() {
		o := e.Value.(*Order)
		if o.Exchange == exchange && o.Symbol == symbol && o.ContractType == contractType {
			ret = append(ret, o)
		}
	}
	return ret
}

// 没有的话创建一个
func (k *keeper) GetPos(exchange string, symbol string, contractType string) *Pos {
	idx := k.findPos(exchange, symbol, contractType)
	if idx >= 0 {
		return k.pos[idx]
	}
	p := &Pos{
		Exchange:     exchange,
		Symbol:       symbol,
		ContractType: contractType,
		IsValid:      true,
	}
	p.Reset()
	k.pos = append(k.pos, p)
	return p
}

func (k *keeper) GetMoney(exchange string, symbol string) *Money {
	idx := k.findMoney(exchange, symbol)
	if idx >= 0 {
		return k.moneys[idx]
	}
	m := &Money{
		Exchange: exchange,
		Symbol:   symbol,
		Balance:  0,
		Rights:   0,
	}
	k.moneys = append(k.moneys, m)
	return m
}

func (k *keeper) GetFeedBack() FeedBack {
	return k.feedback
}
