package krang

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	simplejson "github.com/bitly/go-simplejson"
	"github.com/liu2hai/chive/config"
	"github.com/liu2hai/chive/logs"
	"github.com/liu2hai/chive/protocol"
	"github.com/liu2hai/chive/utils"
)

/*
* tsdb 是对influxDB的封装
* 行情在influxdb里是按照一个商品合约类型一个数据库，比如okex_btc_usd_this_week, okex_btc_usd_next_week
* 由于influxdb只提供HTTP接口，在行情写入量大的时候，会频繁创建销毁链接，虽然在http.header里已经设置了
* Keep-Alive选项，但是不知道influxdb怎么实现，会不会在服务完一个请求后，主动关闭链接。所以决定使用一个
* 数据库一个goroutine定时来写，一次批量写入。为了防止阻塞生产行情的goroutine，没有使用buffer chan来通信
* 而是写到一个缓冲区，定时写数据库的goroutine定时来取。
* 原来想用influxdb的自动聚合功能，通过continus query来定时计算MA，但如果没有行情输入的时候也去计算MA的话
* 得出的结果肯定是不正确的，所以还是在程序里定时去取K线来计算MA
 */

const (
	ant_time_interval = 100 * time.Millisecond
	ant_batch_num     = 150
)

type TSDB interface {
	Check(ex string, symbols []string, contractTypes []string)
	Start()
	Close()

	StoreTick(pb *protocol.PBFutureTick)
	StoreKLine(pb *protocol.PBFutureKLine)
	StoreDepth(pb *protocol.PBFutureDepth)
	StoreTrade(pb *protocol.PBFutureTrade)
	StoreIndex(pb *protocol.PBFutureIndex)

	QueryMAGraphic(db string, kl int32) *MaGraphic
}

type influxdb struct {
	addr    string
	client  *http.Client
	dbNames []string
	antm    map[string]*ant
	klm     map[string]*klmem
	bReplay bool
}

type ant struct {
	addr     string
	db       string
	contents []string
	m        sync.Mutex
	exitCh   chan struct{}
	client   *http.Client
}

func NewTSDBClient(bReplay bool) TSDB {
	return &influxdb{
		addr:    config.T.InfluxDB.Addr,
		client:  &http.Client{},
		dbNames: []string{},
		antm:    make(map[string]*ant),
		klm:     make(map[string]*klmem),
		bReplay: bReplay,
	}
}

/*
 检查influxdb里这些数据库是否存在，不存在就创建
 在influxdb里，一个商品合约作为一个数据库存储行情，
 数据库名格式为：exchange_symbol_contractType
 比如：okex_btc_usd_this_week
*/
func (t *influxdb) Check(ex string, symbols []string, contractTypes []string) {
	names := []string{}
	for _, s := range symbols {
		for _, c := range contractTypes {
			str := utils.MakeupSinfo(ex, s, c)
			names = append(names, str)
		}
	}
	t.dbNames = append(t.dbNames, names...)

	miss := []string{}
	dbs := t.ShowDatabase()
	tm := make(map[string]int)
	for _, n := range dbs {
		tm[n] = 1
	}

	for _, n := range names {
		_, ok := tm[n]
		if !ok {
			miss = append(miss, n)
		}
	}

	for _, n := range miss {
		t.CreateDatabase(n)
	}
}

func (t *influxdb) Start() {
	for _, n := range t.dbNames {
		_, ok := t.antm[n]
		if ok {
			continue
		}

		a := &ant{
			addr:     t.addr,
			db:       n,
			contents: []string{},
			exitCh:   make(chan struct{}),
			client:   &http.Client{},
		}
		t.antm[n] = a
		t.klm[n] = NewKLineMem(n)

		go antLoop(a)
	}
}

func (t *influxdb) Close() {
	for _, v := range t.antm {
		close(v.exitCh)
	}
}

func (t *influxdb) QueryMAGraphic(db string, kl int32) *MaGraphic {
	k, ok := t.klm[db]
	if !ok {
		return nil
	}
	return k.queryMaGraphic(kl)
}

/*
 组织的内容请参考influxdb的line protocol
 weather,tt=121323 temperature=82 1465839830100400200


 时序数据库是以timestamp来做primary key的，key相同的话会被后面写入的覆盖掉
 现在okex的行情是ms为时间戳，但有可能几个ticker的时间戳是相同的，所以这里的
 timestamp使用本地纳秒timestamp来作为primary key，我们行情里的timestamp暂时不使用
*/
func makeInsertSql(table string, fieldm map[string]float32, timestamp uint64) string {
	if len(fieldm) <= 0 {
		return ""
	}

	sql := strings.TrimSpace(table) + " "
	fields := ""

	for k, v := range fieldm {
		fields += k + "=" + fmt.Sprintf("%f", v) + ","
	}
	bs := []byte(fields)
	sql += string(bs[0:len(bs)-1]) + " " + fmt.Sprintf("%d", time.Now().UnixNano())
	return sql
}

func (t *influxdb) insertToAntm(table string, fieldm map[string]float32, sinfo *protocol.PBQuoteSymbol) {
	if sinfo == nil || t.bReplay {
		return
	}

	sql := makeInsertSql(table, fieldm, sinfo.GetTimestamp())
	dbn := utils.MakeupSinfo(sinfo.GetExchange(), sinfo.GetSymbol(), sinfo.GetContractType())
	ant, ok := t.antm[dbn]
	if !ok {
		return
	}

	ant.m.Lock()
	ant.contents = append(ant.contents, sql)
	ant.m.Unlock()
}

func (t *influxdb) StoreTick(pb *protocol.PBFutureTick) {
	table := "ticker"
	fieldm := map[string]float32{
		"vol":     pb.GetVol(),
		"high":    pb.GetHigh(),
		"low":     pb.GetLow(),
		"dayVol":  pb.GetDayVol(),
		"dayHigh": pb.GetDayHigh(),
		"dayLow":  pb.GetDayLow(),
		"last":    pb.GetLast(),
		"bid":     pb.GetBid(),
		"ask":     pb.GetAsk(),
		"bidVol":  pb.GetBidVol(),
		"askVol":  pb.GetAskVol(),
	}
	t.insertToAntm(table, fieldm, pb.GetSinfo())
}

func klineTable(e int32) string {
	switch e {
	case protocol.KL1Min:
		return "kl1min"
	case protocol.KL3Min:
		return "kl3min"
	case protocol.KL5Min:
		return "kl5min"
	case protocol.KL15Min:
		return "kl15min"
	case protocol.KL30Min:
		return "kl30min"
	case protocol.KL1H:
		return "kl1h"
	case protocol.KL1D:
		return "kl1d"
	}
	return "kline"
}

func (t *influxdb) StoreKLine(pb *protocol.PBFutureKLine) {
	table := klineTable(pb.GetKind())
	fieldm := map[string]float32{
		"open":  pb.GetOpen(),
		"high":  pb.GetHigh(),
		"low":   pb.GetLow(),
		"close": pb.GetClose(),
		"vol":   pb.GetVol(),
	}
	t.insertToAntm(table, fieldm, pb.GetSinfo())

	dbn := utils.MakeupSinfo(pb.GetSinfo().GetExchange(), pb.GetSinfo().GetSymbol(), pb.GetSinfo().GetContractType())
	k, ok := t.klm[dbn]
	if !ok {
		return
	}
	k.addKLine(pb)
}

func (t *influxdb) StoreDepth(pb *protocol.PBFutureDepth) {

}

// bscode -- 0 buy 1 sell
func (t *influxdb) StoreTrade(pb *protocol.PBFutureTrade) {
	table := "trade"
	bsc := 0
	if pb.GetBsCode() == "s" {
		bsc = 1
	}
	fieldm := map[string]float32{
		"price":  pb.GetPrice(),
		"vol":    pb.GetVol(),
		"bsCode": float32(bsc),
		"amount": float32(pb.GetAmount()),
	}
	t.insertToAntm(table, fieldm, pb.GetSinfo())
}

func (t *influxdb) StoreIndex(pb *protocol.PBFutureIndex) {
	table := "index"
	fieldm := map[string]float32{
		"futureIndex": pb.GetFutureIndex(),
	}
	t.insertToAntm(table, fieldm, pb.GetSinfo())
}

//////////////////////////////////////////////////////////////////////////////////////////////////////

func antLoop(a *ant) {
	tc := time.NewTimer(ant_time_interval)
	defer tc.Stop()

	for {
		select {
		case <-tc.C:
			tc.Reset(ant_time_interval)
			doAntWork(a)

		case <-a.exitCh:
			return
		}
	}
}

func doAntWork(a *ant) {
	pendings := []string{}
	a.m.Lock()
	pendings = a.contents
	a.contents = make([]string, 0, 32)
	a.m.Unlock()

	count := 0
	body := ""

	// precision设置为ns，因为本地时间戳使用ns
	url := a.addr + "/write?precision=ns&db=" + a.db

	for _, c := range pendings {
		count++
		body += c + "\n"
		if count >= ant_batch_num {
			doHttpReq(a.client, "POST", url, body)
			count = 0
			body = ""
		}
	}
	if len(body) > 0 {
		doHttpReq(a.client, "POST", url, body)
	}
}

func doHttpReq(client *http.Client, method string, url string, b string) *simplejson.Json {
	//fmt.Println("url: ", url)
	//fmt.Println("body len: ", len(b))
	//fmt.Println("body: ", b)

	req, err := http.NewRequest(method, url, strings.NewReader(b))
	if err != nil {
		logs.Error("构建request出错")
		return nil
	}
	req.Header.Set("Content-type", "application/x-www-form-urlencoded")
	req.Header.Set("Connection", "Keep-Alive")

	rsp, err := client.Do(req)
	if err != nil {
		logs.Error("服务器无回应")
		return nil
	}
	defer rsp.Body.Close()

	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		logs.Error("无法得到服务器回应的body")
		return nil
	}

	//fmt.Println("服务器回应: ", rsp.StatusCode, string(body))
	// 204 no content 写influxdb时返回成功的回应
	if rsp.StatusCode != http.StatusOK && rsp.StatusCode != http.StatusNoContent {
		logs.Error("HTTP POST返回状态码错误[%d], body:%s", rsp.StatusCode, string(body))
		return nil
	}

	if rsp.StatusCode == http.StatusNoContent {
		return nil
	}

	js, err := simplejson.NewJson(body)
	if err != nil {
		logs.Error("HTTP POST返回内容不是合法json: %s", string(body))
		return nil
	}
	return js
}

////////////////////////////////////////////////////////////////////////////////////////////////////

func addDoubleQuote(str string) string {
	return "\"" + str + "\""
}

func makeParamStr(params map[string]string) string {
	url := ""
	for k, v := range params {
		url += k + "=" + v + "&"
	}
	bs := []byte(url)
	return string(bs[0 : len(bs)-1])
}

/*
 读取influxdb返回json中的value，
  条件是只有一个stamt被成功执行的时候
  其格式如下：
  {
    "results": [
        {
            "statement_id": 0,
            "series": [
                {
                    "name": "cpu_load_short",
                    "columns": [
                        "time",
                        "value"
                    ],
                    "values": [
                        [
                            "2015-01-29T21:55:43.702900257Z",
                            2
                        ],
                        [
                            "2015-01-29T21:55:43.702900257Z",
                            0.55
                        ],
                        [
                            "2015-06-11T20:46:02Z",
                            0.64
                        ]
                    ]
                }
            ]
        }
    ]
}
*/
func fetchValues(js *simplejson.Json) (*simplejson.Json, int) {
	subjs := js.Get("results").GetIndex(0)
	values := subjs.Get("series").GetIndex(0).Get("values")
	arr, err := values.Array()
	if err != nil {
		return nil, 0
	}
	return values, len(arr)
}

func (t *influxdb) CreateDatabase(name string) bool {
	url := t.addr + "/query"
	stmt := "q=CREATE DATABASE " + addDoubleQuote(name)

	js := doHttpReq(t.client, "POST", url, stmt)
	return js != nil
}

func (t *influxdb) ShowDatabase() []string {
	ret := []string{}
	url := t.addr + "/query"
	stmt := "q=SHOW DATABASES"

	js := doHttpReq(t.client, "POST", url, stmt)
	if js != nil {
		v, l := fetchValues(js)
		if l <= 0 {
			return ret
		}

		for i := 0; i < l; i++ {
			name := v.GetIndex(i).GetIndex(0)
			ret = append(ret, name.MustString())
		}
	}
	return ret
}
