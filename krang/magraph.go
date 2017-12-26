package krang

import (
	"container/list"

	"github.com/liu2hai/chive/logs"
	"github.com/liu2hai/chive/protocol"
	"github.com/liu2hai/chive/utils"
)

const (
	max_maline_len = 10000

	FCS_NONE     = 0 // 快线和慢线无交叉
	FCS_DOWN2TOP = 1 // 快线从下穿越慢线
	FCS_TOP2DOWN = 2 // 快线从上穿越慢线
)

/*
 ma 线交叉点
*/
type MaCrossPoint struct {
	Ts  int64
	Val float32
	Fcs int32
}

// ma线的单个点
type mast struct {
	ts  int64
	val float32
}

// ma线
type maline struct {
	ma     *list.List // mast的列表
	length int32      // 列表当前长度
}

func (l *maline) update(sum float32, tsn int64) {
	ma := mast{
		val: sum,
		ts:  tsn,
	}
	l.ma.PushBack(ma)
	l.length += 1
	if l.length > max_maline_len {
		l.ma.Remove(l.ma.Front())
		l.length -= 1
	}
}

/*
 取ma line 倒数第N个点, N从0开始计数
*/
func (l *maline) back(n int32) (mast, bool) {
	ma := mast{val: -1}
	if l.length <= 0 || n >= l.length || n < 0 {
		return ma, false
	}

	e := l.ma.Back()
	var i int32 = 0
	for i = 0; i != n; i++ {
		e = e.Prev()
	}
	ev := e.Value.(mast)
	ma.val = ev.val
	ma.ts = ev.ts
	return ma, true
}

/*
 取ma line 在[tsStart, tsEnd]时间段内的头尾两点
*/
func (l *maline) segment(tsStart int64, tsEnd int64) (mast, mast, bool) {
	head := mast{val: -1.0}
	tail := mast{val: -1.0}
	for e := l.ma.Front(); e != nil; e = e.Next() {
		ev := e.Value.(mast)
		if ev.ts > tsEnd || ev.ts < tsStart {
			continue
		}

		if head.val < 0 {
			head = ev
			continue
		}

		if tail.val < 0 {
			tail = ev
		}
	}

	if head.val > 0 && tail.val > 0 {
		return head, tail, true
	}
	return head, tail, false
}

////////////////////////////////////////////////////////////////////////////////////////////////////

type MaGraphic struct {
	cpts   *list.List // cross points
	cptlen int32

	//ma线数据
	ma7  maline
	ma30 maline

	klkind int32

	// 最新K线时间
	kts int64
}

func NewMaGraphic() *MaGraphic {
	m := &MaGraphic{}
	m.cpts = list.New()
	m.cptlen = 0
	m.ma7.ma = list.New()
	m.ma7.length = 0
	m.ma30.ma = list.New()
	m.ma30.length = 0
	m.klkind = protocol.KL15Min // m默认使用15分钟k线
	m.kts = 0
	return m
}

////////////////////////////////////////////////////////////////////////////////////////////////////

func (g *MaGraphic) UpdateMa7Line(sum float32, tsn int64) {
	g.ma7.update(sum, tsn)
}

func (g *MaGraphic) UpdateMa30Line(sum float32, tsn int64) {
	g.ma30.update(sum, tsn)
}

// 测试是否有交叉点
// 就取ma7和ma30的最新点，判断是否相等，相等则是在最新时间相交了
func (g *MaGraphic) TryCrossPoint() {
	p7e, b1 := g.ma7.back(0)
	p7s, b2 := g.ma7.back(1)

	p30e, b3 := g.ma30.back(0)
	p30s, b4 := g.ma30.back(1)
	if !b1 || !b2 || !b3 || !b4 {
		return
	}

	cp, ok := getIntersect(p7s, p7e, p30s, p30e)
	if !ok {
		return
	}

	///debug
	logs.Info("[%s] new crosspoint, value[%f], time[%s], fcs[%s]", utils.KLineStr(g.klkind), cp.Val, utils.TSStr(cp.Ts), fcsstr(cp.Fcs))
	///

	g.cpts.PushBack(cp)
	g.cptlen += 1
	if g.cptlen > max_maline_len {
		g.cpts.Remove(g.cpts.Front())
		g.cptlen -= 1
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////

// 取最新K线的时间
func (g *MaGraphic) GetLastKLTimeStamp() int64 {
	return g.kts
}

// 查找最近的交叉点
func (g *MaGraphic) FindLastCrossPoint() (MaCrossPoint, bool) {
	cp := MaCrossPoint{}
	if g.cptlen <= 0 {
		return cp, false
	}
	e := g.cpts.Back().Value.(MaCrossPoint)
	cp.Ts = e.Ts
	cp.Val = e.Val
	cp.Fcs = e.Fcs
	return cp, true
}

// 按时间段查找交叉点
// 从最新的开始找，返回一个列表
func (g *MaGraphic) FindCrossPoints(tsStart int64, tsEnd int64) []MaCrossPoint {
	ret := []MaCrossPoint{}
	if tsEnd < tsStart {
		return ret
	}
	for e := g.cpts.Back(); e != nil; e = e.Prev() {
		m := e.Value.(MaCrossPoint)
		if m.Ts < tsStart {
			break
		}
		if m.Ts >= tsStart && m.Ts <= tsEnd {
			ret = append(ret, m)
		}
	}
	return ret
}

// 计算ma7在某个时间段的斜率
func (g *MaGraphic) ComputeMa7SlopeFactor(tsStart int64, tsEnd int64) float32 {
	head, tail, ok := g.ma7.segment(tsStart, tsEnd)
	if !ok {
		return 0
	}
	return g.slopeFactor(head, tail)
}

// 计算ma30在某个时间段的斜率
func (g *MaGraphic) ComputeMa30SlopeFactor(tsStart int64, tsEnd int64) float32 {
	head, tail, ok := g.ma30.segment(tsStart, tsEnd)
	if !ok {
		return 0
	}
	return g.slopeFactor(head, tail)
}

////////////////////////////////////////////////////////////////////////////////////////////////////

func (g *MaGraphic) SegmentSecs() int64 {
	var t int64 = 1
	if g.klkind == protocol.KL5Min {
		t = 5 * 60 // 5min = 300s
	} else if g.klkind == protocol.KL15Min {
		t = 15 * 60
	}
	return t
}

// x轴为时间，y轴为值
func (g *MaGraphic) slopeFactor(head mast, tail mast) float32 {
	deltaX := tail.ts - head.ts
	if deltaX == 0 {
		return 0
	}

	// 因为mast里存的是时间戳，second，所以转为x坐标时，其实是K线的格数
	deltaX = deltaX / g.SegmentSecs()

	deltaY := tail.val - head.val
	k := deltaY / float32(deltaX)
	return k
}

func fcsstr(fcs int32) string {
	switch fcs {
	case FCS_NONE:
		return "无交叉"
	case FCS_DOWN2TOP:
		return "快线从下穿越慢线"
	case FCS_TOP2DOWN:
		return "快线从上穿越慢线"
	}
	return "未知"
}

/*
	求2个线段相交，线段(a, b)和线段(c, d)
    https://segmentfault.com/a/1190000004457595
*/
type vector struct {
	start mast
	end   mast
	x     float32
	y     float32
}

func newVector(s mast, e mast) vector {
	return vector{
		start: s,
		end:   e,
		x:     float32(e.ts - s.ts),
		y:     e.val - s.val,
	}
}

func negativeVector(v vector) vector {
	return newVector(v.end, v.start)
}

func vectorProduct(va vector, vb vector) float32 {
	return va.x*vb.y - vb.x*va.y
}

/*
 只判断相交，不求具体的相交点，相交点就用2根线段的中点吧
*/
func getIntersect(a, b, c, d mast) (MaCrossPoint, bool) {
	ac := newVector(a, c)
	ad := newVector(a, d)
	bc := newVector(b, c)
	bd := newVector(b, d)

	ca := negativeVector(ac)
	cb := negativeVector(bc)
	da := negativeVector(ad)
	db := negativeVector(bd)

	f1 := vectorProduct(ac, ad)*vectorProduct(bc, bd) <= 0.0000001
	f2 := vectorProduct(ca, cb)*vectorProduct(da, db) <= 0.0000001

	cp := MaCrossPoint{}
	if f1 && f2 {
		dx := b.ts - a.ts
		dy := b.val - a.val

		cp.Ts = a.ts + dx/2
		cp.Val = a.val + dy/2

		cp.Fcs = FCS_NONE
		if a.val < c.val {
			cp.Fcs = FCS_DOWN2TOP
		}
		if a.val >= c.val {
			cp.Fcs = FCS_TOP2DOWN
		}
		return cp, true
	}
	return cp, false
}
