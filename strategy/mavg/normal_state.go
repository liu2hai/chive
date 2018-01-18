package mavg

import (
	"github.com/liu2hai/chive/krang"
	"github.com/liu2hai/chive/logs"
	"github.com/liu2hai/chive/protocol"
	"github.com/liu2hai/chive/strategy"
	"github.com/liu2hai/chive/utils"
)

const (
	LOSSTIMES_STEP = 5
)

/*
 mavg策略的正常状态
 正常状态下，不会加仓
 1. 盈亏状态，当前仓位的盈亏曲线变化
 2. MA信号
 3. 上一个仓位结束后，新建仓位的要冷静一段时间
*/

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

	opTimes     int32   // 操作次数
	profitTimes int32   // 盈利次数
	profitVol   float32 // 盈利币数量
	lossTimes   int32   // 亏损次数
	lossVol     float32 // 亏损币数量
}

type normalState struct {
	spm            map[string]*symbolParam // key:symbol
	opTimes        int32                   // 总操作次数
	profitTimes    int32                   // 总盈利次数
	lossTimes      int32                   // 总亏损次数
	lossTimesLimit int32                   // 总亏损次数限制
}

func NewNormalState() strategy.FSMState {
	ltcParam := &symbolParam{
		klkind:         protocol.KL5Min,
		stopLoseRate:   -0.14,
		stopProfitRate: 0.17,
		minVol:         0.1,
		maxVol:         0.5,
		stepRate:       0.1,
		marketSt:       true,
		level:          10,
	}
	etcParam := &symbolParam{
		klkind:         protocol.KL15Min,
		stopLoseRate:   -0.15,
		stopProfitRate: 0.20,
		minVol:         1,
		maxVol:         5,
		stepRate:       0.1,
		marketSt:       true,
		level:          10,
	}

	st := &normalState{
		spm:            make(map[string]*symbolParam),
		lossTimesLimit: LOSSTIMES_STEP,
	}
	st.spm["ltc_usd"] = ltcParam
	st.spm["etc_usd"] = etcParam
	return st
}

/////////////////////////////////////////////////////

func (t *normalState) Name() string {
	return STATE_NAME_NORMAL
}

func (t *normalState) Init() {
}

func (t *normalState) Enter(ctx krang.Context) {
	logs.Info("进入状态[%s]", t.Name())

	// 重新进入normal state，增加限制
	t.lossTimesLimit += LOSSTIMES_STEP

	// 重新读取头寸信息
	mavg.queryAllPos(ctx)
}

/*
 1. 判断当前是否有仓位
 2. 如果没有仓位，且信号是买，买多
 3. 如果没有仓位，信号是卖，做空
 4. 如果有仓位，normal state不加仓，可以考虑从这里跳转到radical state
 5. 如果有仓位，是否超出止盈止损范围，如果超出，平仓
 6. 如果有仓位，信号是买，空头头寸平仓
 7. 如果有仓位，信号是卖，多头头寸平仓

 平仓或者建仓后，应先更新头寸，因为下单到查询头寸，更新头寸信息，这里有一个时间差
 在这个时间差内应该按照最新的头寸信息操作
*/
func (t *normalState) Decide(ctx krang.Context, tick *krang.Tick, evc *strategy.EventCompose) string {
	t.handleLongPart(ctx, tick, evc)
	t.handleShortPart(ctx, tick, evc)

	if evc.HasEmergency() {
		logs.Info("紧急情况, 策略暂时关闭")
		return STATE_NAME_SHUTDOWN
	}
	return t.Name()
}

/////////////////////////////////////////////////////

func (t *normalState) getSymbolParam(symbol string) *symbolParam {
	v, ok := t.spm[symbol]
	if !ok {
		return nil
	}
	return v
}

func (t *normalState) handleLongPart(ctx krang.Context, tick *krang.Tick, evc *strategy.EventCompose) {
	sp := t.getSymbolParam(tick.Symbol)
	if sp == nil {
		return
	}

	s, ok := evc.Macd.Signals[sp.klkind]
	if !ok || evc.Pos == nil {
		return
	}

	if evc.Pos.LongAvai <= 0 {
		if s == strategy.SIGNAL_BUY {
			reason := "买入信号"
			t.doOpenPos(ctx, tick, evc, protocol.ORDERTYPE_OPENLONG, reason, sp)
		}
		return
	}

	// 有多头头寸情况
	if s == strategy.SIGNAL_EMERGENCY {
		reason := "紧急情况"
		t.doClosePos(ctx, tick, evc, protocol.ORDERTYPE_CLOSELONG, reason, sp)
		return
	}

	if evc.Pos.LongFloatPRate <= sp.stopLoseRate || evc.Pos.LongFloatPRate >= sp.stopProfitRate {
		reason := "超出止盈止损范围"
		t.doClosePos(ctx, tick, evc, protocol.ORDERTYPE_CLOSELONG, reason, sp)
		return
	}

	if s == strategy.SIGNAL_SELL {
		reason := "卖出信号，平多"
		t.doClosePos(ctx, tick, evc, protocol.ORDERTYPE_CLOSELONG, reason, sp)
	}
}

func (t *normalState) handleShortPart(ctx krang.Context, tick *krang.Tick, evc *strategy.EventCompose) {
	sp := t.getSymbolParam(tick.Symbol)
	if sp == nil {
		return
	}

	s, ok := evc.Macd.Signals[sp.klkind]
	if !ok || evc.Pos == nil {
		return
	}

	if evc.Pos.ShortAvai <= 0 {
		if s == strategy.SIGNAL_SELL {
			reason := "卖出信号"
			t.doOpenPos(ctx, tick, evc, protocol.ORDERTYPE_OPENSHORT, reason, sp)
		}
		return
	}

	// 有空头头寸情况
	if s == strategy.SIGNAL_EMERGENCY {
		reason := "紧急情况"
		t.doClosePos(ctx, tick, evc, protocol.ORDERTYPE_CLOSESHORT, reason, sp)
		return
	}

	if evc.Pos.ShortFloatPRate <= sp.stopLoseRate || evc.Pos.ShortFloatPRate >= sp.stopProfitRate {
		reason := "超出止盈止损范围"
		t.doClosePos(ctx, tick, evc, protocol.ORDERTYPE_CLOSESHORT, reason, sp)
		return
	}

	if s == strategy.SIGNAL_BUY {
		reason := "买入信号，平空"
		t.doClosePos(ctx, tick, evc, protocol.ORDERTYPE_CLOSESHORT, reason, sp)
	}
}

func (t *normalState) doOpenPos(ctx krang.Context, tick *krang.Tick, evc *strategy.EventCompose, ot int32, reason string, sp *symbolParam) {
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
	t.opTimes += 1
	sp.opTimes += 1

	logs.Info("策略mavg开仓, [%s_%s_%s], 合约张数[%d], 币数量[%f], 订单类型[%s], 杠杆[%d], 原因[%s]",
		cmd.Exchange, cmd.Symbol, cmd.ContractType, cmd.Amount, cmd.Vol, utils.OrderTypeStr(cmd.OrderType), sp.level, reason)
}

func (t *normalState) doClosePos(ctx krang.Context, tick *krang.Tick, evc *strategy.EventCompose, ot int32, reason string, sp *symbolParam) {
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
	t.updateStatics(cmd.Symbol, profit, sp)
}

func (t *normalState) updateStatics(symbol string, profit float32, sp *symbolParam) {
	t.opTimes += 1
	sp.opTimes += 1
	if profit > 0 {
		t.profitTimes += 1
		sp.profitTimes += 1
		sp.profitVol += profit
	} else {
		t.lossTimes += 1
		sp.lossTimes += 1
		sp.lossVol += (profit * -1)
	}
	logs.Info("策略mavg统计，[%s]盈利次数[%d], 币量[%f]，亏损次数[%d]，币量[%f]", symbol, sp.profitTimes, sp.profitVol, sp.lossTimes, sp.lossVol)
}
