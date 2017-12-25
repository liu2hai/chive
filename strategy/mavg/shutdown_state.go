package mavg

import (
	"time"

	"github.com/liu2hai/chive/krang"
	"github.com/liu2hai/chive/strategy"
)

/*
 mavg策略的关闭状态
 关闭状态下，该策略不会再下单，等待管理端命令
 或者行情好转而恢复正常
*/

type shutdownState struct {
	ts int64 // 上次关闭的时间
}

func NewShutdownState() strategy.FSMState {
	return &shutdownState{}
}

func (t *shutdownState) Name() string {
	return STATE_NAME_SHUTDOWN
}

func (t *shutdownState) Init() {
	t.ts = time.Now().Unix()
}

func (t *shutdownState) Enter() {
	t.ts = time.Now().Unix()
}

// 关闭后，暂停一个小时后重开
func (t *shutdownState) Decide(ctx krang.Context, tick *krang.Tick, evc *strategy.EventCompose) string {
	n := time.Now()
	old := time.Unix(t.ts, 0)
	d := n.Sub(old)
	if d.Hours() >= 1.0 {
		return STATE_NAME_NORMAL
	}

	return t.Name()
}
