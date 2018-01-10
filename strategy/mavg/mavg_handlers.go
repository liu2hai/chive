package mavg

import (
	"fmt"

	"github.com/liu2hai/chive/krang"
	"github.com/liu2hai/chive/logs"
	"github.com/liu2hai/chive/protocol"
	"github.com/liu2hai/chive/strategy"
	"github.com/liu2hai/chive/utils"
)

//////////////////////////////////////
// 盈亏和仓位计算
type posHandler struct {
}

func NewPosHandler() strategy.FSMHandler {
	return &posHandler{}
}

func (p *posHandler) Name() string {
	return "profit_handler"
}

func (p *posHandler) OnTick(ctx krang.Context, tick *krang.Tick, e *strategy.EventCompose) {
	kp := ctx.GetKeeper()
	pos := kp.GetPos(tick.Exchange, tick.Symbol, tick.ContractType)
	if pos == nil {
		panic("keeper internal error")
	}
	printPos(pos)

	money := kp.GetMoney(tick.Exchange, tick.Symbol)
	if money == nil {
		panic("keeper internal error")
	}

	if !pos.IsValid {
		logs.Info("[%s_%s_%s]等待头寸更新", pos.Exchange, pos.Symbol, pos.ContractType)
		e.Pos = nil
		return
	}

	e.Pos = pos
	e.Money.Balance = money.Balance
}

func printPos(pos *krang.Pos) {
	fmt.Println("-------------")
	fmt.Println(pos.Symbol, pos.ContractType)
	fmt.Println("多头合约张数：", pos.LongAmount)
	fmt.Println("多头可平合约张数: ", pos.LongAvai)
	fmt.Println("多头保证金: ", pos.LongBond)
	fmt.Println("多头强平价格：", pos.LongFlatPrice)
	fmt.Println("多头开仓平均价：", pos.LongPriceAvg)
	fmt.Println("多头结算基准价：", pos.LongPriceCost)
	fmt.Println("多头平仓盈亏：", pos.LongCloseProfit)
	fmt.Println("多头浮动盈亏：", pos.LongFloatProfit)
	fmt.Println("多头浮动盈亏比例：", pos.LongFloatPRate)
	fmt.Println(" ")
	fmt.Println("空头合约张数：", pos.ShortAmount)
	fmt.Println("空头可平合约张数: ", pos.ShortAvai)
	fmt.Println("空头保证金: ", pos.ShortBond)
	fmt.Println("空头开仓平均价：", pos.ShortPriceAvg)
	fmt.Println("空头结算基准价：", pos.ShortPriceCost)
	fmt.Println("空头平仓盈亏：", pos.ShortCloseProfit)
	fmt.Println("空头浮动盈亏：", pos.ShortFloatProfit)
	fmt.Println("空头浮动盈亏比例：", pos.ShortFloatPRate)
	fmt.Println("-------------")
}

//////////////////////////////////////////////////////////////////////////
/*
  使用指定K线的MA7和MA30指标
  快线 -- MA7, 慢线 -- MA30

  1. 快线和慢线的大斜率确定方向，权重最高
  2. 快线从下穿越慢线，买进信号；快线从上穿越慢线，卖出信号
  3. 确定快线和慢线这段时间的差距变化
  4. 确定交叉点的疏密程度，也就在某个时间段内的交叉点个数，说明趋势是否明显
*/

type klParam struct {
	klkind   int32
	distance int32
	unit     int32
	fkrate   float32 // 快线斜率
	skrate   float32 // 慢线斜率
	dkrate   float32 // 快线和慢线差距曲线的斜率
	fsdiff   float32 // 快线和慢线的差距值
}

type macdHandler struct {
	k5mp  *klParam
	k15mp *klParam
}

func NewMACDHandler() strategy.FSMHandler {
	p5 := &klParam{
		klkind:   protocol.KL5Min, // 使用5分钟k线
		distance: 3,               // 使用N根k线计算斜率
		unit:     5 * 60,
		fkrate:   0.18,
		skrate:   0.2,
		dkrate:   0.1,
		fsdiff:   1.0,
	}
	p15 := &klParam{
		klkind:   protocol.KL15Min,
		distance: 3,
		unit:     15 * 60,
		fkrate:   0.15,
		skrate:   0.04,
		dkrate:   0.03,
		fsdiff:   1.1,
	}

	return &macdHandler{
		k5mp:  p5,
		k15mp: p15,
	}
}

func (m *macdHandler) Name() string {
	return "macd_handler"
}

func (m *macdHandler) getKlParam(klkind int32) *klParam {
	if klkind == protocol.KL5Min {
		return m.k5mp
	} else if klkind == protocol.KL15Min {
		return m.k15mp
	}
	return nil
}

func (m *macdHandler) OnTick(ctx krang.Context, tick *krang.Tick, e *strategy.EventCompose) {
	m.doTick(ctx, tick, e, protocol.KL5Min)
	m.doTick(ctx, tick, e, protocol.KL15Min)
}

func (m *macdHandler) doTick(ctx krang.Context, tick *krang.Tick, e *strategy.EventCompose, klkind int32) {
	e.Macd.Signals[klkind] = 0
	db := utils.MakeupSinfo(e.Exchange, e.Symbol, e.ContractType)
	g := ctx.GetQuoteDB().QueryMAGraphic(db, klkind)
	kp := m.getKlParam(klkind)
	strKind := utils.KLineStr(klkind)
	if g == nil || kp == nil {
		return
	}

	// 取最新K线的时间，这样在回测里才有效
	tsn := g.GetLastKLTimeStamp()
	tickTime := int64(tick.Timestamp / 1000)
	if tsn <= 0 || tickTime <= 0 {
		return
	}

	// tick的时间和kline的时间相差太多，说明kline没有及时更新
	// 实际运行过程中发现，有时服务器会停止推送kline
	du := tickTime - tsn
	if du > int64(kp.unit*2) || du < int64(kp.unit*-1) {
		logs.Error("[%s-%s]tick时间[%s]和最新kline时间[%s]相差太多，策略应停止。", tick.Symbol, strKind,
			utils.TSStr(tickTime), utils.TSStr(tsn))
		e.Macd.Signals[klkind] = strategy.SIGNAL_EMERGENCY
		return
	}

	tsStart := tsn - int64(kp.distance*kp.unit)
	ma7Slope := g.ComputeMa7SlopeFactor(tsStart, tsn)
	ma30Slope := g.ComputeMa30SlopeFactor(tsStart, tsn)
	diffSlope := g.ComputeDiffSlopeFactor(tsStart, tsn)
	fsdiff := g.GetLastDiff()

	if utils.IsZero32(ma7Slope) || utils.IsZero32(ma30Slope) || utils.IsZero32(fsdiff) {
		return
	}

	///debug
	logs.Info("[%s-%s]macd signal, tsn[%s], ma7Slope[%f], ma30Slope[%f], diffSlope[%f], fsdiff[%f]",
		tick.Symbol, strKind, utils.TSStr(tsn), ma7Slope, ma30Slope, diffSlope, fsdiff)
	///

	cp, ok := g.FindLastCrossPoint()
	if !ok {
		m.noCrossPointCase(ma7Slope, ma30Slope, diffSlope, e, klkind)
		return
	}

	///debug
	logs.Info("[%s-%s]macd signal, cp val[%f], cp time[%s]", tick.Symbol, strKind, cp.Val, utils.TSStr(cp.Ts))
	///

	/*
	  首先判断趋势是否成立，发出建仓信号，买多信号对于空头就是平仓信号
	*/

	// 斜率> 0 , 往右上斜，就是趋势向上
	if ma7Slope >= kp.fkrate && ma30Slope > kp.skrate {
		if cp.Fcs == protocol.FCS_DOWN2TOP && fsdiff >= kp.fsdiff {
			e.Macd.Signals[klkind] = strategy.SIGNAL_BUY
			logs.Info("[%s-%s]产生买信号K1", tick.Symbol, strKind)
			return
		}
	}

	// 斜率< 0 , 往右下斜，就是趋势向下
	if ma7Slope <= (-1*kp.fkrate) && ma30Slope < (-1*kp.skrate) {
		if cp.Fcs == protocol.FCS_TOP2DOWN && fsdiff >= kp.fsdiff {
			e.Macd.Signals[klkind] = strategy.SIGNAL_SELL
			logs.Info("[%s-%s]产生卖信号K2", tick.Symbol, strKind)
			return
		}
	}

	/*
	  其次要判断拐点的到来
	  快线向下，慢线向上
	  快线向上，慢线向下
	*/
	if ma7Slope < (-1*kp.fkrate) && diffSlope < (-1*kp.dkrate) {
		e.Macd.Signals[klkind] = strategy.SIGNAL_SELL
		logs.Info("[%s-%s]产生卖信号K3", tick.Symbol, strKind)
		return
	}

	if ma7Slope > kp.fkrate && ma30Slope <= (-1*kp.skrate) && diffSlope < (-1*kp.dkrate) {
		e.Macd.Signals[klkind] = strategy.SIGNAL_BUY
		logs.Info("[%s-%s]产生买信号K4", tick.Symbol, strKind)
		return
	}
}

// 在diffSlope变大的时候，也就是快线和慢线差距变大的时候，才考虑下单
func (m *macdHandler) noCrossPointCase(ma7Slope float32, ma30Slope float32, diffSlope float32, e *strategy.EventCompose, klkind int32) {
	kp := m.getKlParam(klkind)
	strKind := utils.KLineStr(klkind)
	if kp == nil {
		return
	}

	if diffSlope <= kp.dkrate {
		return
	}

	// 斜率> 0 , 往右上斜，就是趋势向上
	if ma7Slope >= kp.fkrate && ma30Slope > kp.skrate {
		e.Macd.Signals[klkind] = strategy.SIGNAL_BUY
		logs.Info("[%s-%s]产生买信号K5", e.Symbol, strKind)
		return
	}

	// 斜率< 0 , 往右下斜，就是趋势向下
	if ma7Slope <= (-1*kp.fkrate) && ma30Slope < (-1*kp.skrate) {
		e.Macd.Signals[klkind] = strategy.SIGNAL_SELL
		logs.Info("[%s-%s]产生买信号K6", e.Symbol, strKind)
		return
	}
}
