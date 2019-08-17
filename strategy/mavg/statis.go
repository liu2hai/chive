package mavg

import (
	"chive/logs"
)

/*
 做一些统计工作
*/

const (
	LOSSTIMES_STEP = 5
)

type symStatis struct {
	opTimes     int32   // 操作次数
	openTimes   int32   // 建仓次数
	closeTimes  int32   // 平仓次数
	profitTimes int32   // 盈利次数
	profitVol   float32 // 盈利币数量
	lossTimes   int32   // 亏损次数
	lossVol     float32 // 亏损币数量
}

type totalStatis struct {
	opTimes        int32                 // 总操作次数
	openTimes      int32                 // 总建仓次数
	closeTimes     int32                 // 总平仓次数
	profitTimes    int32                 // 总盈利次数
	lossTimes      int32                 // 总亏损次数
	lossTimesLimit int32                 // 总亏损次数限制
	m              map[string]*symStatis // 各个商品的统计
}

var mavgStatis *totalStatis

////////////////////////////////////////////////////////////////////////////////////////////////////

func InitMavgStatis() {
	mavgStatis = &totalStatis{
		lossTimesLimit: LOSSTIMES_STEP,
		m:              make(map[string]*symStatis),
	}
	mavgStatis.m["ltc_usd"] = &symStatis{}
	mavgStatis.m["etc_usd"] = &symStatis{}
}

func UpLossTimesLimit() {
	mavgStatis.lossTimesLimit += LOSSTIMES_STEP
}

func IsOverLossLimit() bool {
	return mavgStatis.lossTimes >= mavgStatis.lossTimesLimit
}

func UpdateOpenStatis(symbol string) {
	ss, ok := mavgStatis.m[symbol]
	if !ok {
		return
	}
	ss.opTimes += 1
	ss.openTimes += 1
	mavgStatis.opTimes += 1
	mavgStatis.openTimes += 1
}

func UpdateCloseStatis(symbol string, profit float32) {
	ss, ok := mavgStatis.m[symbol]
	if !ok {
		return
	}

	mavgStatis.opTimes += 1
	mavgStatis.closeTimes += 1
	ss.opTimes += 1
	ss.closeTimes += 1

	if profit > 0 {
		mavgStatis.profitTimes += 1
		ss.profitTimes += 1
		ss.profitVol += profit
	} else {
		mavgStatis.lossTimes += 1
		ss.lossTimes += 1
		ss.lossVol += (profit * -1)
	}

	logs.Info("策略mavg统计，[%s]盈利次数[%d], 币量[%f]，亏损次数[%d]，币量[%f]", symbol, ss.profitTimes, ss.profitVol, ss.lossTimes, ss.lossVol)
}
