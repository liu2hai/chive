package mavg

import (
	"github.com/liu2hai/chive/krang"
	"github.com/liu2hai/chive/logs"
	"github.com/liu2hai/chive/strategy"
	"github.com/liu2hai/chive/utils"
)

/*
	简单的移动平均线策略
	策略要实现krang.Strategy的接口
	并且要在RegisStrategy函数里把自己注册进策略管理器

	其实策略应该是一个状态机，由最新行情驱动在各个状态中跳转
*/

type MavgStrategy struct {
	exchange      string
	symbols       []string
	contractTypes []string
	follows       []string
	fsm           *strategy.FSM
}

var mavg = &MavgStrategy{}

// 本策略名称
const THIS_STRATEGY_NAME = "mavg"

// 策略状态名称
const (
	STATE_NAME_SHUTDOWN = "shutdown"
	STATE_NAME_NORMAL   = "normal"
	STATE_NAME_RADICAL  = "radical"
	STATE_NAME_DEFENSE  = "defense"
)

const FB_MAX_CHECKTIMES = 3 // 反馈中未完成命令检查次数

////////////////////////////////////////////////////////////////////////////////////////////////////

/*
	策略初始化函数
*/
func (t *MavgStrategy) Init(ctx krang.Context) {
	logs.Info("mavg strategy is working now ...")

	// 初始化统计数据工作
	InitMavgStatis()

	// 需要关注的交易所和商品合约
	t.exchange = "okex"
	t.symbols = []string{"ltc_usd", "etc_usd"}
	t.contractTypes = []string{"this_week"}
	t.makeupFllows()

	// 初始化FSM
	t.fsm = strategy.NewFSM(THIS_STRATEGY_NAME)

	shutst := NewShutdownState()
	shutst.Init()
	t.fsm.AddState(shutst)

	normalst := NewNormalState()
	normalst.Init()
	t.fsm.AddState(normalst)

	radicalst := NewRadicalState()
	radicalst.Init()
	t.fsm.AddState(radicalst)

	defense := NewDefenseState()
	defense.Init()
	t.fsm.AddState(defense)

	// 默认状态是正常状态
	t.fsm.SetState(STATE_NAME_NORMAL)

	ph := NewPosHandler()
	t.fsm.AddHandler(ph)

	mh := NewMACDHandler()
	t.fsm.AddHandler(mh)

	// 查询全部关注的资金和头寸
	t.queryAllPos(ctx)
}

/*
  检查反馈函数
  可以在这里检测该策略之前的下单和撤单是否成功执行
  然后决定是否继续执行策略
*/
func (t *MavgStrategy) CheckFeedBack(ctx krang.Context) bool {
	fb := ctx.GetKeeper().GetFeedBack()
	datas := fb.FindByStrategy(THIS_STRATEGY_NAME)
	if len(datas) <= 0 {
		return true
	}

	for _, v := range datas {
		logs.Info("[%s]策略回馈中没有执行完成的命令：tid[%d], reqserial[%d]", THIS_STRATEGY_NAME, v.Tid, v.ReqSerial)
		v.CheckTimes += 1
		if v.CheckTimes >= FB_MAX_CHECKTIMES {
			fb.Remove(v.ReqSerial)
		}

		// 没有反馈都去及时更新头寸
		t.queryAllPos(ctx)
	}
	return true
}

/*
  行情更新函数
  驱动状态机运转
*/
func (t *MavgStrategy) OnTick(ctx krang.Context, tick *krang.Tick) {
	if !t.isFitStrategy(tick) {
		return
	}

	t.fsm.Call(ctx, tick)
}

/*
	将本策略注册到krang的策略管理器里
*/
func RegisStrategy() {
	krang.RegisterStrategy(THIS_STRATEGY_NAME, mavg)
}

////////////////////////////////////////////////////////////////////////////////////////////////////

func (t *MavgStrategy) isFitStrategy(tick *krang.Tick) bool {
	target := utils.MakeupSinfo(tick.Exchange, tick.Symbol, tick.ContractType)
	for _, v := range t.follows {
		if v == target {
			return true
		}
	}
	return false
}

func (t *MavgStrategy) makeupFllows() {
	for _, s := range t.symbols {
		for _, c := range t.contractTypes {
			tmp := utils.MakeupSinfo(t.exchange, s, c)
			t.follows = append(t.follows, tmp)
		}
	}
}

func (t *MavgStrategy) queryAllPos(ctx krang.Context) {
	trader := ctx.GetTrader(t.exchange)
	if trader == nil {
		return
	}

	trader.QueryAccount()
	for _, s := range t.symbols {
		for _, c := range t.contractTypes {
			trader.QueryPos(s, c)
		}
	}
}
