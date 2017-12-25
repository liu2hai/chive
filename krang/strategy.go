package krang

type Strategy interface {

	/*
		策略初始化函数
	*/
	Init(ctx Context)

	/*
	  检查反馈函数,该函数会在OnTick前被调用
	  如果返回false，则OnTick不会被调用
	*/
	CheckFeedBack(ctx Context) bool

	/*
	  行情更新函数
	*/
	OnTick(ctx Context, tick *Tick)
}

type StrategyManager struct {
	m map[string]Strategy
}

var stmgr = &StrategyManager{
	m: make(map[string]Strategy),
}

func GetStrategyMgr() *StrategyManager {
	return stmgr
}

func RegisterStrategy(name string, st Strategy) {
	if name == "" || st == nil {
		panic("RegisterStrategy panic, parameter error")
	}

	if _, dup := stmgr.m[name]; dup {
		panic("Register one Strategy for twice")
	}
	stmgr.m[name] = st
}
