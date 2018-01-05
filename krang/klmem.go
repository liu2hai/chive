package krang

import (
	"container/list"

	"github.com/liu2hai/chive/logs"
	"github.com/liu2hai/chive/protocol"
	"github.com/liu2hai/chive/utils"
)

const max_rbf_len = 10000

/*
 k line 在内存的存储
*/

type klst struct {
	open   float32
	high   float32
	low    float32
	close  float32
	vol    float32
	amount float32
	ts     int64
}

type rbf struct {
	kl  *list.List
	kc  int32 // total count
	mag *MaGraphic
}

type klmem struct {
	db  string
	k5  *rbf
	k15 *rbf
}

func newrbf() *rbf {
	return &rbf{
		kl:  list.New(),
		kc:  0,
		mag: NewMaGraphic(),
	}
}

func NewKLineMem(db string) *klmem {
	kl := &klmem{
		db:  db,
		k5:  newrbf(),
		k15: newrbf(),
	}
	kl.k5.mag.klkind = protocol.KL5Min
	kl.k15.mag.klkind = protocol.KL15Min
	return kl
}

func (k *klmem) addKLine(pb *protocol.PBFutureKLine) int32 {
	if pb.GetKind() == protocol.KL5Min {
		return addImpl(k.k5, pb)
	} else if pb.GetKind() == protocol.KL15Min {
		return addImpl(k.k15, pb)
	}
	return 0
}

func (k *klmem) getLastKLine(kind int32) (klst, bool) {
	if kind == protocol.KL5Min {
		return fetchImpl(k.k5, kind)
	} else if kind == protocol.KL15Min {
		return fetchImpl(k.k15, kind)
	}
	return klst{}, false
}

func fetchImpl(r *rbf, kind int32) (klst, bool) {
	if r.kc <= 0 {
		return klst{}, false
	}
	e := r.kl.Back().Value.(klst)
	return e, true
}

func sumRangeKList(l *list.List, num int32) float32 {
	if num <= 0 {
		return 0
	}

	var c int32 = 0
	var sum float32 = 0.0
	for e := l.Back(); e != nil; e = e.Prev() {
		if c >= num {
			break
		}
		k := e.Value.(klst)
		sum += k.close
		c += 1
	}
	return sum / float32(num)
}

/*
  因为通过ws订阅的K线数据，交易所是有变动就发送数据，有可能这个K线没有封闭
  所以在这里来判断新来的K线数据和最新存储的是不是同一根K线
  返回值大于0，表示有新的K线生成
*/
func addImpl(r *rbf, pb *protocol.PBFutureKLine) int32 {
	tsn := int64(pb.GetSinfo().GetTimestamp() / 1000)
	symbol := pb.GetSinfo().GetSymbol()
	k := klst{
		open:   pb.GetOpen(),
		high:   pb.GetHigh(),
		low:    pb.GetLow(),
		close:  pb.GetClose(),
		vol:    pb.GetVol(),
		amount: pb.GetAmount(),
		ts:     tsn,
	}

	if r.kc > 0 {
		lastKL := r.kl.Back().Value.(klst)
		diff := tsn - lastKL.ts
		if diff < 0 {
			// 旧数据，扔掉
			return 0
		}

		if diff >= 0 && diff < r.mag.SegmentSecs() {
			// 同一根K线数据
			r.kl.Remove(r.kl.Back())
			r.kl.PushBack(k)
			return 0
		}
	}

	r.kl.PushBack(k)
	r.kc += 1
	if r.kc > max_rbf_len {
		r.kl.Remove(r.kl.Front())
		r.kc -= 1
	}
	r.mag.kts = tsn

	/// Debug
	logs.Info("[%s-%s] new kline, close[%f], time[%s]", symbol, utils.KLineStr(pb.GetKind()), k.close, utils.TSStr(tsn))
	/// Debug

	var need int32 = 0

	// 计算ma7 和ma30
	var sum7 float32
	if r.kc >= 7 {
		sum7 = sumRangeKList(r.kl, 7)
		r.mag.UpdateMa7Line(sum7, tsn)
		need += 1

		/// Debug
		logs.Info("[%s-%s] new ma7, sum7[%f], time[%s]", symbol, utils.KLineStr(pb.GetKind()), sum7, utils.TSStr(tsn))
		/// Debug
	}

	var sum30 float32
	if r.kc >= 30 {
		sum30 = sumRangeKList(r.kl, 30)
		r.mag.UpdateMa30Line(sum30, tsn)
		need += 1

		/// Debug
		logs.Info("[%s-%s] new ma30, sum30[%f], time[%s]", symbol, utils.KLineStr(pb.GetKind()), sum30, utils.TSStr(tsn))
		/// Debug
	}

	if need >= 2 {
		delta := sum7 - sum30
		if delta < 0.0 {
			delta = delta * -1
		}
		r.mag.UpdateDiffLine(delta, tsn)
		r.mag.TryCrossPoint()
	}
	return 1
}

func (k *klmem) queryMaGraphic(kline int32) *MaGraphic {
	if kline == protocol.KL5Min {
		return k.k5.mag
	} else if kline == protocol.KL15Min {
		return k.k15.mag
	}
	return nil
}
