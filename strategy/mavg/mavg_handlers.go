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
		return
	}
	printPos(pos)

	money := kp.GetMoney(tick.Exchange, tick.Symbol)
	if money == nil {
		return
	}

	e.Pos = pos
	e.Money.Balance = money.Balance
}

func printPos(pos *krang.Pos) {
	fmt.Println("-------------")
	fmt.Println(pos.Symbol, pos.ContractType)
	fmt.Println("多头合约张数：%f", pos.LongAmount)
	fmt.Println("多头可平合约张数: %f", pos.LongAvai)
	fmt.Println("多头保证金: %f", pos.LongBond)
	fmt.Println("多头强平价格：%f", pos.LongFlatPrice)
	fmt.Println("多头开仓平均价：%f", pos.LongPriceAvg)
	fmt.Println("多头结算基准价：%f", pos.LongPriceCost)
	fmt.Println("多头平仓盈亏：%f", pos.LongCloseProfit)
	fmt.Println("多头浮动盈亏：%f", pos.LongFloatProfit)
	fmt.Println("多头浮动盈亏比例：%f", pos.LongFloatPRate)
	fmt.Println(" ")
	fmt.Println("空头合约张数：%f", pos.ShortAmount)
	fmt.Println("空多头可平合约张数: %f", pos.ShortAvai)
	fmt.Println("空多头保证金: %f", pos.ShortBond)
	fmt.Println("空多头开仓平均价：%f", pos.ShortPriceAvg)
	fmt.Println("空多头结算基准价：%f", pos.ShortPriceCost)
	fmt.Println("空多头平仓盈亏：%f", pos.ShortCloseProfit)
	fmt.Println("空多头浮动盈亏：%f", pos.ShortFloatProfit)
	fmt.Println("空多头浮动盈亏比例：%f", pos.ShortFloatPRate)
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
}

func NewMACDHandler() strategy.FSMHandler {
	return &macdHandler{
		klkind:   protocol.KL5Min, // 使用5分钟k线
		distance: 15,              // 使用50根k线计算斜率
		unit:     5 * 60,
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

	cp, ok := g.FindLastCrossPoint()
	if !ok {
		return
	}

	// 取最新K线的时间，这样在回测里才有效
	tsn := g.GetLastKLTimeStamp()
	tsStart := tsn - int64(m.distance*m.unit)
	ma7Slope := g.ComputeMa7SlopeFactor(tsStart, tsn)
	ma30Slope := g.ComputeMa30SlopeFactor(tsStart, tsn)

	///debug
	logs.Info("macd signal, ma7Slope[%f], ma30Slope[%f], cp val[%f], cp time[%s]", ma7Slope, ma30Slope, cp.Val, utils.TSStr(cp.Ts))
	///

	// 斜率> 0 , 往右上斜，就是趋势向上
	if ma7Slope >= 1 && ma30Slope > 0.4 {
		if cp.Fcs == krang.FCS_DOWN2TOP {
			e.Macd.Signals[m.klkind] = strategy.SIGNAL_BUY

			///debug
			logs.Info("产生买信号")
			///
		}
	}

	// 斜率< 0 , 往右下斜，就是趋势向下
	if ma7Slope <= -1 && ma30Slope < -0.4 {
		if cp.Fcs == krang.FCS_TOP2DOWN {
			e.Macd.Signals[m.klkind] = strategy.SIGNAL_SELL

			///debug
			logs.Info("产生卖信号")
			///
		}
	}
}
