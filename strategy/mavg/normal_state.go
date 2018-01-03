package mavg

import (
	"github.com/liu2hai/chive/krang"
	"github.com/liu2hai/chive/logs"
	"github.com/liu2hai/chive/protocol"
	"github.com/liu2hai/chive/strategy"
	"github.com/liu2hai/chive/utils"
)

/*
 mavg策略的正常状态
 正常状态下，不会加仓
 1. 盈亏状态，当前仓位的盈亏曲线变化
 2. MA信号
 3. 上一个仓位结束后，新建仓位的要冷静一段时间
*/

type normalState struct {
	stopLoseRate   float32 // 止损率
	stopProfitRate float32 // 止盈率
	minVol         float32 // 最小仓位，币
	maxVol         float32 // 最大仓位，币
	stepRate       float32 //单次下单占仓位比例
	marketSt       bool    // 是否使用市价单
	openTimes      int32   // 开仓次数
	openTimesLimit int32   // 开仓次数限制
}

func NewNormalState() strategy.FSMState {
	return &normalState{
		stopLoseRate:   -0.1,
		stopProfitRate: 0.15,
		minVol:         0.1,
		maxVol:         0.5,
		stepRate:       0.01,
		marketSt:       true,
		openTimes:      0,
		openTimesLimit: 5,
	}
}

func (t *normalState) Name() string {
	return STATE_NAME_NORMAL
}

func (t *normalState) Init() {
}

func (t *normalState) Enter() {
}

/*
 1. 判断当前是否有仓位
 2. 如果没有仓位，且信号是买，买多
 3. 如果没有仓位，信号是卖，做空
 4. 如果有仓位，normal state不加仓，可以考虑从这里跳转到radical state
 5. 如果有仓位，是否超出止盈止损范围，如果超出，平仓
 6. 如果有仓位，信号是买，空头头寸平仓
 7. 如果有仓位，信号是卖，多头头寸平仓
*/
func (t *normalState) Decide(ctx krang.Context, tick *krang.Tick, evc *strategy.EventCompose) string {
	t.handleLongPart(ctx, tick, evc)
	t.handleShortPart(ctx, tick, evc)
	return t.Name()
}

func (t *normalState) handleLongPart(ctx krang.Context, tick *krang.Tick, evc *strategy.EventCompose) {
	s, ok := evc.Macd.Signals[protocol.KL5Min]
	if !ok {
		return
	}
	if evc.Pos == nil {
		return
	}

	if evc.Pos.LongAmount <= 0 {
		if s == strategy.SIGNAL_BUY {
			t.doOpenPos(ctx, tick, evc, protocol.ORDERTYPE_OPENLONG)
		}
	} else {
		if evc.Pos.LongFloatPRate <= t.stopLoseRate || evc.Pos.LongFloatPRate >= t.stopLoseRate {
			t.doClosePos(ctx, tick, evc, protocol.ORDERTYPE_CLOSELONG)
		}

		if s == strategy.SIGNAL_SELL {
			t.doClosePos(ctx, tick, evc, protocol.ORDERTYPE_CLOSELONG)
		}
	}
}

func (t *normalState) handleShortPart(ctx krang.Context, tick *krang.Tick, evc *strategy.EventCompose) {
	s, ok := evc.Macd.Signals[protocol.KL5Min]
	if !ok {
		return
	}
	if evc.Pos == nil {
		return
	}

	if evc.Pos.ShortAmount <= 0 {
		if s == strategy.SIGNAL_SELL {
			t.doOpenPos(ctx, tick, evc, protocol.ORDERTYPE_OPENSHORT)
		}
	} else {
		if evc.Pos.ShortFloatPRate <= t.stopLoseRate || evc.Pos.ShortFloatPRate >= t.stopLoseRate {
			t.doClosePos(ctx, tick, evc, protocol.ORDERTYPE_CLOSESHORT)
		}

		if s == strategy.SIGNAL_BUY {
			t.doClosePos(ctx, tick, evc, protocol.ORDERTYPE_CLOSESHORT)
		}
	}
}

func (t *normalState) doOpenPos(ctx krang.Context, tick *krang.Tick, evc *strategy.EventCompose, ot int32) {
	trader := ctx.GetTrader(evc.Exchange)
	if trader == nil {
		return
	}
	if t.openTimes >= t.openTimesLimit {
		logs.Info("已达到开仓次数限制")
		return
	}

	// 使用对手价，价格填0
	var price float32 = 0
	var pst = protocol.PRICE_ST_MARKET
	if !t.marketSt {
		price = tick.Last
		pst = protocol.PRICE_ST_LIMIT
	}

	// 计算合约张数
	vol := evc.Money.Balance * t.stepRate
	if vol < t.minVol {
		vol = t.minVol
	}
	if vol > t.maxVol {
		vol = t.maxVol
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
		Level:        10,
		Vol:          vol,
	}
	trader.SetOrder(cmd)
	t.openTimes += 1

	logs.Info("策略mavg开仓, [%s_%s_%s], 合约张数[%d], 币数量[%f], 订单类型[%s], 杠杆[%d]",
		cmd.Exchange, cmd.Symbol, cmd.ContractType, cmd.Amount, cmd.Vol, utils.OrderTypeStr(cmd.OrderType), 10)
}

func (t *normalState) doClosePos(ctx krang.Context, tick *krang.Tick, evc *strategy.EventCompose, ot int32) {
	trader := ctx.GetTrader(evc.Exchange)
	if trader == nil {
		return
	}

	var amount int32 = 0
	var bond float32 = 0
	if ot == protocol.ORDERTYPE_CLOSELONG {
		amount = int32(evc.Pos.LongAvai)
		bond = evc.Pos.LongBond
	}
	if ot == protocol.ORDERTYPE_CLOSESHORT {
		amount = int32(evc.Pos.ShortAvai)
		bond = evc.Pos.ShortBond
	}

	cmd := krang.SetOrderCmd{
		Stname:       THIS_STRATEGY_NAME,
		Exchange:     evc.Exchange,
		Symbol:       evc.Symbol,
		ContractType: evc.ContractType,
		Price:        0,
		Amount:       amount,
		OrderType:    ot,
		PriceSt:      protocol.PRICE_ST_MARKET,
		Level:        10,
		Vol:          0,
	}
	trader.SetOrder(cmd)
	logs.Info("策略mavg平仓, [%s_%s_%s], 合约张数[%d], 币数量[%f], 订单类型[%s], 杠杆[%d]",
		cmd.Exchange, cmd.Symbol, cmd.ContractType, cmd.Amount, bond, utils.OrderTypeStr(cmd.OrderType), 10)
}
