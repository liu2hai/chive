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
	fmt.Println("空多头可平合约张数: ", pos.ShortAvai)
	fmt.Println("空多头保证金: ", pos.ShortBond)
	fmt.Println("空多头开仓平均价：", pos.ShortPriceAvg)
	fmt.Println("空多头结算基准价：", pos.ShortPriceCost)
	fmt.Println("空多头平仓盈亏：", pos.ShortCloseProfit)
	fmt.Println("空多头浮动盈亏：", pos.ShortFloatProfit)
	fmt.Println("空多头浮动盈亏比例：", pos.ShortFloatPRate)
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

type macdHandler struct {
	klkind   int32
	distance int32
	unit     int32
	fkrate   float32 // 快线斜率
	skrate   float32 // 慢线斜率
	dkrate   float32 // 快线和慢线差距曲线的斜率
	fsdiff   float32 // 快线和慢线的差距值
}

func NewMACDHandler() strategy.FSMHandler {
	return &macdHandler{
		klkind:   protocol.KL5Min, // 使用5分钟k线
		distance: 3,               // 使用N根k线计算斜率
		unit:     5 * 60,
		fkrate:   0.1,
		skrate:   0.04,
		dkrate:   0.03,
		fsdiff:   1.2,
	}
}

func (m *macdHandler) Name() string {
	return "macd_handler"
}

func (m *macdHandler) OnTick(ctx krang.Context, tick *krang.Tick, e *strategy.EventCompose) {
	e.Macd.Signals[m.klkind] = 0
	db := utils.MakeupSinfo(e.Exchange, e.Symbol, e.ContractType)
	g := ctx.GetQuoteDB().QueryMAGraphic(db, m.klkind)
	if g == nil {
		return
	}

	// 取最新K线的时间，这样在回测里才有效
	tsn := g.GetLastKLTimeStamp()
	tsStart := tsn - int64(m.distance*m.unit)
	ma7Slope := g.ComputeMa7SlopeFactor(tsStart, tsn)
	ma30Slope := g.ComputeMa30SlopeFactor(tsStart, tsn)
	diffSlope := g.ComputeDiffSlopeFactor(tsStart, tsn)
	fsdiff := g.GetLastDiff()

	if utils.IsZero32(ma7Slope) || utils.IsZero32(ma30Slope) || utils.IsZero32(fsdiff) {
		return
	}

	///debug
	logs.Info("macd signal, tsn[%s], ma7Slope[%f], ma30Slope[%f], diffSlope[%f], fsdiff[%f]", utils.TSStr(tsn), ma7Slope, ma30Slope, diffSlope, fsdiff)
	///

	cp, ok := g.FindLastCrossPoint()
	if !ok {
		m.noCrossPointCase(ma7Slope, ma30Slope, diffSlope, e)
		return
	}

	///debug
	logs.Info("macd signal, cp val[%f], cp time[%s]", cp.Val, utils.TSStr(cp.Ts))
	///

	/*
	  首先判断趋势是否成立，发出建仓信号，买多信号对于空头就是平仓信号
	*/

	// 斜率> 0 , 往右上斜，就是趋势向上
	if ma7Slope >= m.fkrate && ma30Slope > m.skrate {
		if cp.Fcs == protocol.FCS_DOWN2TOP && fsdiff >= m.fsdiff {
			e.Macd.Signals[m.klkind] = strategy.SIGNAL_BUY

			///debug
			logs.Info("产生买信号K1")
			///
		}
	}

	// 斜率< 0 , 往右下斜，就是趋势向下
	if ma7Slope <= (-1*m.fkrate) && ma30Slope < (-1*m.skrate) {
		if cp.Fcs == protocol.FCS_TOP2DOWN && fsdiff >= m.fsdiff {
			e.Macd.Signals[m.klkind] = strategy.SIGNAL_SELL

			///debug
			logs.Info("产生卖信号K2")
			///
		}
	}

	/*
	  其次要判断拐点的到来
	*/
	if ma7Slope < (-1*m.fkrate) && ma30Slope > 0 && diffSlope < m.dkrate {
		e.Macd.Signals[m.klkind] = strategy.SIGNAL_SELL
		///debug
		logs.Info("产生卖信号K3")
		///
	}

	if ma7Slope > m.fkrate && ma30Slope < 0 && diffSlope < m.dkrate {
		e.Macd.Signals[m.klkind] = strategy.SIGNAL_SELL
		///debug
		logs.Info("产生卖信号K4")
		///
	}
}

// 在diffSlope变大的时候，也就是快线和慢线差距变大的时候，才考虑下单
func (m *macdHandler) noCrossPointCase(ma7Slope float32, ma30Slope float32, diffSlope float32, e *strategy.EventCompose) {
	if diffSlope <= m.dkrate {
		return
	}

	// 斜率> 0 , 往右上斜，就是趋势向上
	if ma7Slope >= m.fkrate && ma30Slope > m.skrate {
		e.Macd.Signals[m.klkind] = strategy.SIGNAL_BUY

		///debug
		logs.Info("产生买信号K5")
		///
	}

	// 斜率< 0 , 往右下斜，就是趋势向下
	if ma7Slope <= (-1*m.fkrate) && ma30Slope < (-1*m.skrate) {
		e.Macd.Signals[m.klkind] = strategy.SIGNAL_SELL

		///debug
		logs.Info("产生卖信号K6")
		///
	}
}
