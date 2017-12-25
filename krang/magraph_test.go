package krang

import (
	"fmt"
	"testing"
)

func TestIntersect(t *testing.T) {
	// 不相交
	a := mast{ts: 3, val: 278}
	b := mast{ts: 5, val: 284}
	c := mast{ts: 6, val: 256}
	d := mast{ts: 10, val: 303}

	cp, ok := getIntersect(a, b, c, d)
	if ok {
		fmt.Println("has intersect, cp[ts, val, fcs]", cp.Ts, cp.Val, cp.Fcs)
	} else {
		fmt.Println("no intersect, cp[ts, val, fcs]", cp.Ts, cp.Val, cp.Fcs)
	}

	// 相交
	a.ts = 3
	a.val = 280
	b.ts = 7
	b.val = 263

	c.ts = 2
	c.val = 233
	d.ts = 9
	d.val = 316

	cp2, ok := getIntersect(a, b, c, d)
	if ok {
		fmt.Println("has intersect, cp[ts, val, fcs]", cp2.Ts, cp2.Val, cp2.Fcs)
	} else {
		fmt.Println("no intersect, cp[ts, val, fcs]", cp2.Ts, cp2.Val, cp2.Fcs)
	}

	// 平行
	a.ts = 3
	a.val = 280
	b.ts = 7
	b.val = 280

	c.ts = 2
	c.val = 233
	d.ts = 12
	d.val = 233

	cp3, ok := getIntersect(a, b, c, d)
	if ok {
		fmt.Println("has intersect, cp[ts, val, fcs]", cp3.Ts, cp3.Val, cp3.Fcs)
	} else {
		fmt.Println("no intersect, cp[ts, val, fcs]", cp3.Ts, cp3.Val, cp3.Fcs)
	}

	// 重合
	a.ts = 3
	a.val = 280
	b.ts = 7
	b.val = 280

	c.ts = 3
	c.val = 280
	d.ts = 7
	d.val = 280

	cp4, ok := getIntersect(a, b, c, d)
	if ok {
		fmt.Println("has intersect, cp[ts, val, fcs]", cp4.Ts, cp4.Val, cp4.Fcs)
	} else {
		fmt.Println("no intersect, cp[ts, val, fcs]", cp4.Ts, cp4.Val, cp4.Fcs)
	}
}
