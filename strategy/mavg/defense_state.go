package mavg

import (
	"time"

	"github.com/liu2hai/chive/krang"
	"github.com/liu2hai/chive/logs"
	"github.com/liu2hai/chive/protocol"
	"github.com/liu2hai/chive/strategy"
)

/*
 防御状态，只平仓不建仓
 平仓参数倾向于保守
*/

type defenseState struct {
	spm   map[string]*symbolParam // key:symbol
	ts    int64                   // 上次进入该状态的时间
	times int32                   // 进本状态次数
}

func NewDefenseState() strategy.FSMState {
	ltcParam := &symbolParam{
		klkind:         protocol.KL5Min,
		stopLoseRate:   -0.10,
		stopProfitRate: 0.10,
		minVol:         0,
		maxVol:         0,
		stepRate:       0,
		marketSt:       true,
		level:          10,
	}
	etcParam := &symbolParam{
		klkind:         protocol.KL1Min,
		stopLoseRate:   -0.1,
		stopProfitRate: 0.10,
		minVol:         0,
		maxVol:         0,
		stepRate:       0,
		marketSt:       true,
		level:          10,
	}

	st := &defenseState{
		spm:   make(map[string]*symbolParam),
		ts:    0,
		times: 0,
	}
	st.spm["ltc_usd"] = ltcParam
	st.spm["etc_usd"] = etcParam
	return st
}

func (t *defenseState) Name() string {
	return STATE_NAME_DEFENSE
}

func (t *defenseState) Init() {
}

func (t *defenseState) Enter(ctx krang.Context) {
	t.ts = time.Now().Unix()
	t.times += 1

	// 重新读取头寸信息
	mavg.queryAllPos(ctx)
	logs.Info("亏损次数[%d]达到限制[%d]，进入状态[%s], 进入次数[%d]", mavgStatis.lossTimes, mavgStatis.lossTimesLimit, t.Name(), t.times)
}

func (t *defenseState) Decide(ctx krang.Context, tick *krang.Tick, evc *strategy.EventCompose) string {
	t.handleLongPart(ctx, tick, evc)
	t.handleShortPart(ctx, tick, evc)

	n := time.Now()
	old := time.Unix(t.ts, 0)
	d := n.Sub(old)
	if d.Hours() >= 1 {
		return STATE_NAME_NORMAL
	}
	return t.Name()
}

////////////////////////////////////////////////////////////////////////////////////////////////////

func (t *defenseState) getSymbolParam(symbol string) *symbolParam {
	v, ok := t.spm[symbol]
	if !ok {
		return nil
	}
	return v
}

func (t *defenseState) handleLongPart(ctx krang.Context, tick *krang.Tick, evc *strategy.EventCompose) {
	sp := t.getSymbolParam(tick.Symbol)
	if sp == nil {
		return
	}

	s, ok := evc.Macd.Signals[sp.klkind]
	if !ok || evc.Pos == nil {
		return
	}

	if evc.Pos.LongAvai <= 0 {
		return
	}

	// 有多头头寸情况
	if s == strategy.SIGNAL_EMERGENCY {
		reason := "紧急情况"
		ArcherClosePos(ctx, tick, evc, protocol.ORDERTYPE_CLOSELONG, reason, sp)
		return
	}

	if evc.Pos.LongFloatPRate <= sp.stopLoseRate || evc.Pos.LongFloatPRate >= sp.stopProfitRate {
		reason := "超出止盈止损范围"
		ArcherClosePos(ctx, tick, evc, protocol.ORDERTYPE_CLOSELONG, reason, sp)
		return
	}
}

func (t *defenseState) handleShortPart(ctx krang.Context, tick *krang.Tick, evc *strategy.EventCompose) {
	sp := t.getSymbolParam(tick.Symbol)
	if sp == nil {
		return
	}

	s, ok := evc.Macd.Signals[sp.klkind]
	if !ok || evc.Pos == nil {
		return
	}

	if evc.Pos.ShortAvai <= 0 {
		return
	}

	// 有空头头寸情况
	if s == strategy.SIGNAL_EMERGENCY {
		reason := "紧急情况"
		ArcherClosePos(ctx, tick, evc, protocol.ORDERTYPE_CLOSESHORT, reason, sp)
		return
	}

	if evc.Pos.ShortFloatPRate <= sp.stopLoseRate || evc.Pos.ShortFloatPRate >= sp.stopProfitRate {
		reason := "超出止盈止损范围"
		ArcherClosePos(ctx, tick, evc, protocol.ORDERTYPE_CLOSESHORT, reason, sp)
		return
	}
}
