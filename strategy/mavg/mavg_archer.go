package mavg

/*
开仓平仓操作
*/

import (
	"github.com/liu2hai/chive/krang"
	"github.com/liu2hai/chive/logs"
	"github.com/liu2hai/chive/protocol"
	"github.com/liu2hai/chive/strategy"
	"github.com/liu2hai/chive/utils"
)

// 针对单个商品设置的参数
type symbolParam struct {
	klkind         int32   // 使用的K线信号
	stopLoseRate   float32 // 止损率
	stopProfitRate float32 // 止盈率
	minVol         float32 // 最小仓位，币
	maxVol         float32 // 最大仓位，币
	stepRate       float32 // 单次下单占仓位比例
	marketSt       bool    // 是否使用市价单
	level          int32   // 本策略支持的杠杆
}

func ArcherOpenPos(ctx krang.Context, tick *krang.Tick, evc *strategy.EventCompose, ot int32, reason string, sp *symbolParam) {
	trader := ctx.GetTrader(evc.Exchange)
	if trader == nil {
		return
	}

	// 使用对手价，价格填0
	var price float32 = 0
	var pst = protocol.PRICE_ST_MARKET
	if !sp.marketSt {
		price = tick.Last
		pst = protocol.PRICE_ST_LIMIT
	}

	// 计算合约张数
	vol := evc.Money.Balance * sp.stepRate
	if vol < sp.minVol {
		vol = sp.minVol
	}
	if vol > sp.maxVol {
		vol = sp.maxVol
	}
	amount := trader.ComputeContractAmount(evc.Symbol, tick.Last, vol)
	if amount <= 0 {
		return
	}

	cmd := krang.SetOrderCmd{
		Stname:       THIS_STRATEGY_NAME,
		Exchange:     evc.Exchange,
		Symbol:       evc.Symbol,
		ContractType: evc.ContractType,
		Price:        price,
		Amount:       amount,
		OrderType:    ot,
		PriceSt:      int32(pst),
		Level:        sp.level,
		Vol:          vol,
	}
	trader.SetOrder(cmd)
	evc.Pos.Disable()
	UpdateOpenStatis(evc.Symbol)

	logs.Info("策略mavg开仓, [%s_%s_%s], 合约张数[%d], 币数量[%f], 订单类型[%s], 杠杆[%d], 原因[%s]",
		cmd.Exchange, cmd.Symbol, cmd.ContractType, cmd.Amount, cmd.Vol, utils.OrderTypeStr(cmd.OrderType), sp.level, reason)
}

func ArcherClosePos(ctx krang.Context, tick *krang.Tick, evc *strategy.EventCompose, ot int32, reason string, sp *symbolParam) {
	trader := ctx.GetTrader(evc.Exchange)
	if trader == nil {
		return
	}

	var amount int32 = 0
	var bond float32 = 0
	var rate float32 = 0
	if ot == protocol.ORDERTYPE_CLOSELONG {
		amount = int32(evc.Pos.LongAvai)
		bond = evc.Pos.LongBond
		rate = evc.Pos.LongFloatPRate
	}
	if ot == protocol.ORDERTYPE_CLOSESHORT {
		amount = int32(evc.Pos.ShortAvai)
		bond = evc.Pos.ShortBond
		rate = evc.Pos.ShortFloatPRate
	}
	if amount <= 0 {
		return
	}

	cmd := krang.SetOrderCmd{
		Stname:       THIS_STRATEGY_NAME,
		Exchange:     evc.Exchange,
		Symbol:       evc.Symbol,
		ContractType: evc.ContractType,
		Price:        tick.Last,
		Amount:       amount,
		OrderType:    ot,
		PriceSt:      protocol.PRICE_ST_MARKET,
		Level:        sp.level,
		Vol:          0,
	}
	trader.SetOrder(cmd)
	evc.Pos.Disable()
	profit := bond * rate

	logs.Info("策略mavg平仓, [%s_%s_%s], 合约张数[%d], 币数量[%f], 订单类型[%s], 杠杆[%d], 原因[%s], 预计盈亏[%f, %f]",
		cmd.Exchange, cmd.Symbol, cmd.ContractType, cmd.Amount, bond, utils.OrderTypeStr(cmd.OrderType), sp.level, reason,
		rate*100, profit)

	// 盈亏统计
	UpdateCloseStatis(evc.Symbol, profit)
}
