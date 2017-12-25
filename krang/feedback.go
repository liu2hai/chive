package krang

type FeedBack interface {
	Add(stname string, reqSerial uint32, tid uint32, data string)
	Remove(reqSerial uint32)
	FindByStrategy(stname string) []*FeedData
}

type FeedData struct {
	Stname    string
	ReqSerial uint32
	Tid       uint32
	Data      string
}

type feedback struct {
	m map[uint32]*FeedData
}

func NewFeedBack() FeedBack {
	return &feedback{
		m: make(map[uint32]*FeedData),
	}
}

func (t *feedback) Add(stname string, reqSerial uint32, tid uint32, data string) {
	_, ok := t.m[reqSerial]
	if ok {
		return
	}
	d := &FeedData{
		Stname:    stname,
		ReqSerial: reqSerial,
		Tid:       tid,
		Data:      data,
	}
	t.m[reqSerial] = d
}

func (t *feedback) Remove(reqSerial uint32) {
	_, ok := t.m[reqSerial]
	if !ok {
		return
	}
	delete(t.m, reqSerial)
}

func (t *feedback) FindByStrategy(stname string) []*FeedData {
	ret := []*FeedData{}
	for _, v := range t.m {
		if v.Stname == stname {
			ret = append(ret, v)
		}
	}
	return ret
}
