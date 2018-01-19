package mavg

import (
	"github.com/liu2hai/chive/krang"
	"github.com/liu2hai/chive/strategy"
)

/*
 防御状态，只平仓不建仓
 平仓参数倾向于保守
*/

type defenseState struct {
}

func NewDefenseState() strategy.FSMState {
	return &defenseState{}
}

func (t *defenseState) Name() string {
	return STATE_NAME_DEFENSE
}

func (t *defenseState) Init() {
}

func (t *defenseState) Enter(ctx krang.Context) {
}

func (t *defenseState) Decide(ctx krang.Context, tick *krang.Tick, evc *strategy.EventCompose) string {
	return t.Name()
}
