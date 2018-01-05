package front

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/liu2hai/chive/logs"
	"github.com/liu2hai/chive/protocol"
	"github.com/liu2hai/chive/utils"

	simplejson "github.com/bitly/go-simplejson"
	"github.com/gorilla/websocket"
)

const (
	hbInterval = 20 //发送心跳间隔，单位second
	cmdExit    = -1
)

type okexQuoter struct {
	wsurl   string
	errm    map[int]string
	symbols []string
	kinds   []string
}

func newOKExQuoter() ExchangeQuote {
	return &okexQuoter{
		wsurl: "wss://real.okex.com:10440/websocket/okexapi",
		errm:  make(map[int]string),
		/*
			symbols: []string{"btc", "ltc"},
			kinds:   []string{"this_week", "next_week", "quarter", "index"},
		*/
		symbols: []string{"ltc", "etc"},
		kinds:   []string{"this_week"},
	}
}

func (t *okexQuoter) Init() error {
	utils.InitOkexErrorMap(t.errm)
	return nil
}

func (t *okexQuoter) Run() {
	for _, s := range t.symbols {
		for _, k := range t.kinds {
			go t.RunImpl(s, k)
		}
	}
}

/*
  主协程开出读写2个协程，并监控他们是否退出，只要有一个退出
  主协程会结束链接，这2个协程遇到链接结束肯定会退出，主协程重新来过
*/
func (t *okexQuoter) RunImpl(symbol string, kind string) {
	for {
		c := utils.Reconnect(t.wsurl, "okex", "quote")
		rgc := make(chan int)
		wgc := make(chan int)

		go readLoop(c, rgc, symbol, kind)
		go writeLoop(c, wgc, symbol, kind)

	L:
		for {
			select {
			case _, ok := <-rgc:
				if !ok {
					break L
				}
			case _, ok := <-wgc:
				if !ok {
					break L
				}
			}
		}
		c.Close()
		logs.Error("okex run %s_%s restart.... ", symbol, kind)
	}
}

func readLoop(c *websocket.Conn, rgc chan int, symbol string, kind string) {
	defer close(rgc)
	var getSubReply = false
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			logs.Error("%s_%s sub ws error read:%s", symbol, kind, err.Error())
			return
		}
		fmt.Println("ws收到通知: ", string(message))

		// 去除心跳回应
		if len(message) == len(`{"event":"pong"}`) {
			continue
		}

		js, err := simplejson.NewJson(message)
		if err != nil {
			logs.Error("%s_%s sub ws parse json error:%s, json: %s", symbol, kind, err.Error(), message)
			return
		}

		if !getSubReply {
			var ec string
			getSubReply, ec = parseSubReply(js)
			if !getSubReply {
				logs.Error("%s_%s sub ws reply error code:%s, json: %s", symbol, kind, ec, message)
				return
			}
		} else {
			if err := parseSubNtf(symbol, kind, js); err != nil {
				logs.Error("%s_%s sub ws parse ntf error:%s, json: %s", symbol, kind, err.Error(), message)
				return
			}
		}
	}
}

func writeLoop(c *websocket.Conn, wgc chan int, symbol string, kind string) {
	tc := time.NewTimer(hbInterval * time.Second)
	defer tc.Stop()
	defer close(wgc)

	if kind == "index" {
		subContractIndex(c, symbol, kind)
	} else {
		subContractQuote(c, symbol, kind)
	}

	for {
		data := makeHBPket()
		err := c.WriteMessage(websocket.TextMessage, []byte(data))
		if err != nil {
			logs.Error("okex write goroutine write error, %s", err.Error())
			return
		}
		<-tc.C
		tc.Reset(hbInterval * time.Second)
	}
}

func makeHBPket() string {
	return "{'event':'ping'}"
}

func subContractIndex(c *websocket.Conn, symbol string, kind string) {
	// 合约指数
	indexStr := "{'event':'addChannel','channel':'ok_sub_futureusd_%s_index'}"
	symbolIndex := fmt.Sprintf(indexStr, symbol)
	c.WriteMessage(websocket.TextMessage, []byte(symbolIndex))
}

func subContractQuote(c *websocket.Conn, symbol string, kind string) {

	// tiker 数据
	tikerStr := "{'event':'addChannel','channel':'ok_sub_futureusd_%s_ticker_%s'}"
	tikerReq := fmt.Sprintf(tikerStr, symbol, kind)

	// kline 数据，订阅1分钟, 5分钟， 15分钟的k线
	//klineStr1 := "{'event':'addChannel','channel':'ok_sub_futureusd_%s_kline_%s_1min'}"
	//klineReq1 := fmt.Sprintf(klineStr1, symbol, kind)

	klineStr5 := "{'event':'addChannel','channel':'ok_sub_futureusd_%s_kline_%s_5min'}"
	klineReq5 := fmt.Sprintf(klineStr5, symbol, kind)

	klineStr15 := "{'event':'addChannel','channel':'ok_sub_futureusd_%s_kline_%s_15min'}"
	klineReq15 := fmt.Sprintf(klineStr15, symbol, kind)

	// depth 市场深度，全量返回，20档
	//depthStr := "{'event':'addChannel','channel':'ok_sub_futureusd_%s_depth_%s_20'}"
	//depthReq := fmt.Sprintf(depthStr, symbol, kind)

	// 成交回报
	//tradeStr := "{'event':'addChannel','channel':'ok_sub_futureusd_%s_trade_%s'}"
	//tradeReq := fmt.Sprintf(tradeStr, symbol, kind)

	c.WriteMessage(websocket.TextMessage, []byte(tikerReq))
	//c.WriteMessage(websocket.TextMessage, []byte(klineReq1))
	c.WriteMessage(websocket.TextMessage, []byte(klineReq5))
	c.WriteMessage(websocket.TextMessage, []byte(klineReq15))
	//c.WriteMessage(websocket.TextMessage, []byte(tradeReq))
	//c.WriteMessage(websocket.TextMessage, []byte(depthReq))

}

////////////////////////////////////////////////////////////////////////////////////
/*
 解析订阅返回json，订阅回复如下格式：
成功订阅:
 [{"binary":0,"channel":"addChannel","data":{"result":true,"channel":"ok_sub_futureusd_ltc_ticker_this_week"}}]

 订阅失败:
[{"binary":0,"channel":"ok_sub_futureusd_ltc_ffthis_week","data":{"result":false,"error_msg":"param not match.","error_code":20116}}]
*/
func parseSubReply(js *simplejson.Json) (bool, string) {
	arr, err := js.Array()
	if err != nil {
		return false, "format error"
	}
	ll := len(arr)
	for i := 0; i < ll; i++ {
		subjs := js.GetIndex(i)
		ec, ok := subjs.Get("data").CheckGet("error_code")
		if ok {
			errCode, _ := ec.String()
			return false, errCode
		}
	}
	return true, ""
}

/*
 根据订阅的channel来解析不同格式json
 将json转换为对应的pb数据，发往后台
 对于每一个商品的每一个品种，都有下面几类数据
 ticker, kline, depth, trade
 每一个商品有一个index数据
*/
func parseSubNtf(symbol string, kind string, js *simplejson.Json) error {
	arr, err := js.Array()
	if err != nil {
		return err
	}
	ll := len(arr)
	for i := 0; i < ll; i++ {
		subjs := js.GetIndex(i)
		ch := subjs.Get("channel").MustString()
		data := subjs.Get("data")
		if err := parseNtfDetailData(symbol, kind, ch, data); err != nil {
			return err
		}
	}
	return nil
}

// 1min, 3min, 5min, 15min, 30min, 1hour, 2hour, 4hour, 6hour, 12hour, day, 3day, week
func getklkind(ch string) int32 {
	m := map[string]int32{
		"1min":  protocol.KL1Min,
		"3min":  protocol.KL3Min,
		"5min":  protocol.KL5Min,
		"15min": protocol.KL15Min,
		"30min": protocol.KL30Min,
		"1hour": protocol.KL1H,
		"day":   protocol.KL1D,
	}
	for k, v := range m {
		if strings.Contains(ch, k) {
			return v
		}
	}
	return protocol.KL1D
}

/*
	indexStr := "{'event':'addChannel','channel':'ok_sub_futureusd_%s_index'}"
	tikerStr := "{'event':'addChannel','channel':'ok_sub_futureusd_%s_ticker_%s'}"
	klineStr := "{'event':'addChannel','channel':'ok_sub_futureusd_%s_kline_%s_1min'}"
	depthStr := "{'event':'addChannel','channel':'ok_sub_futureusd_%s_depth_%s_60'}"
	tradeStr := "{'event':'addChannel','channel':'ok_sub_futureusd_%s_trade_%s'}"
*/
func parseNtfDetailData(symbol string, kind string, ch string, js *simplejson.Json) error {
	if strings.Contains(ch, "index") {
		return parseNtfIndex(symbol, kind, ch, js)
	}

	if strings.Contains(ch, "ticker") {
		return parseNtfTicker(symbol, kind, ch, js)
	}

	if strings.Contains(ch, "kline") {
		k := getklkind(ch)
		return parseNtfKLine(symbol, kind, ch, js, k)
	}

	if strings.Contains(ch, "depth") {
		return parseNtfDepth(symbol, kind, ch, js)
	}

	if strings.Contains(ch, "trade") {
		return parseNtfTrade(symbol, kind, ch, js)
	}
	return nil
}

/*
 okex的index通知data元素格式如下：
 {"data":{"futureIndex":"3922.26","timestamp":"1505294822049"}
*/
func parseNtfIndex(symbol string, kind string, ch string, js *simplejson.Json) error {
	pb := &protocol.PBFutureIndex{}
	idx, err := strconv.ParseFloat(js.Get("futureIndex").MustString(), 64)
	pb.FutureIndex = proto.Float32(float32(idx))
	tt, err := strconv.ParseUint(js.Get("timestamp").MustString(), 10, 64)
	if err != nil {
		tt = 0
	}

	sinfo := &protocol.PBQuoteSymbol{}
	sinfo.Exchange = proto.String("okex")
	sinfo.Symbol = proto.String(symbol + "_usd")
	sinfo.ContractType = proto.String(kind)
	sinfo.Timestamp = proto.Uint64(tt)
	pb.Sinfo = sinfo
	okexQuoteReply(protocol.FID_QUOTE_Index, pb)
	fmt.Println("index: ", pb.String())
	return nil
}

/*
 okex的tiker格式data元素如下：
  "data": {
		 "high": "170.535",
		 "limitLow": "156.172",
		 "vol": "14047824",
		 "last": "165.123",
		 "low": "138.766",
		 "buy": "165.122",
		 "hold_amount": "1274970",
		 "sell": "165.123",
		 "contractId": 20171215116,
		 "unitAmount": "10",
		 "limitHigh": "167.811"
      }
limitHigh(string):最高买入限制价格
limitLow(string):最低卖出限制价格
vol(double):24小时成交量
sell(double):卖一价格
unitAmount(double):合约价值
hold_amount(double):当前持仓量
contractId(long):合约ID
high(double):24小时最高价格
low(double):24小时最低价格

*/
func parseNtfTicker(symbol string, kind string, ch string, js *simplejson.Json) error {
	pb := &protocol.PBFutureTick{}

	// 24h成交量
	vol, _ := strconv.ParseFloat(js.Get("vol").MustString(), 64)
	pb.DayVol = proto.Float32(float32(vol))

	// 24h最高
	dhigh, _ := strconv.ParseFloat(js.Get("high").MustString(), 64)
	pb.DayHigh = proto.Float32(float32(dhigh))

	// 24h最低
	dlow, _ := strconv.ParseFloat(js.Get("low").MustString(), 64)
	pb.DayLow = proto.Float32(float32(dlow))

	// 最新价
	last, _ := strconv.ParseFloat(js.Get("last").MustString(), 64)
	pb.Last = proto.Float32(float32(last))

	// 买一价
	buy, _ := strconv.ParseFloat(js.Get("buy").MustString(), 64)
	pb.Bid = proto.Float32(float32(buy))

	// 卖一价
	sell, _ := strconv.ParseFloat(js.Get("sell").MustString(), 64)
	pb.Ask = proto.Float32(float32(sell))

	sinfo := &protocol.PBQuoteSymbol{}
	sinfo.Exchange = proto.String("okex")
	sinfo.Symbol = proto.String(symbol + "_usd")
	sinfo.ContractType = proto.String(kind)
	sinfo.Timestamp = proto.Uint64(0)
	pb.Sinfo = sinfo
	okexQuoteReply(protocol.FID_QUOTE_TICK, pb)
	return nil
}

/*
 okex 1min kline的data元素格式如下：
 "data":[["1505298360000","3707.48","3707.48","3692.29","3692.29","80.0","2.160421855325"]]
 [时间 ,开盘价,最高价,最低价,收盘价,成交量(张),成交量(币)]
 [string, string, string, string, string, string]
  数组里可能有多个元素
*/
func parseNtfKLine(symbol string, kind string, ch string, js *simplejson.Json, k int32) error {
	arr, err := js.Array()
	if err != nil {
		return err
	}
	ll := len(arr)
	for i := 0; i < ll; i++ {
		sub := js.GetIndex(i)
		subarr, err := sub.Array()
		if err != nil {
			return err
		}
		if len(subarr) != 7 {
			logs.Error("parseNtfKLine array num error, len[%d]", len(subarr))
			return errors.New("parseNtfKLine array num error")
		}

		pb := &protocol.PBFutureKLine{}
		tt, err := strconv.ParseUint(sub.GetIndex(0).MustString(), 10, 64)
		if err != nil {
			tt = uint64(time.Now().Unix() * 1000)
		}

		open, _ := strconv.ParseFloat(sub.GetIndex(1).MustString(), 64)
		pb.Open = proto.Float32(float32(open))

		high, _ := strconv.ParseFloat(sub.GetIndex(2).MustString(), 64)
		pb.High = proto.Float32(float32(high))

		low, _ := strconv.ParseFloat(sub.GetIndex(3).MustString(), 64)
		pb.Low = proto.Float32(float32(low))

		close, _ := strconv.ParseFloat(sub.GetIndex(4).MustString(), 64)
		pb.Close = proto.Float32(float32(close))

		amount, _ := strconv.ParseFloat(sub.GetIndex(5).MustString(), 64)
		pb.Amount = proto.Float32(float32(amount))

		vol, _ := strconv.ParseFloat(sub.GetIndex(6).MustString(), 64)
		pb.Vol = proto.Float32(float32(vol))

		pb.Kind = proto.Int32(k)

		sinfo := &protocol.PBQuoteSymbol{}
		sinfo.Exchange = proto.String("okex")
		sinfo.Symbol = proto.String(symbol + "_usd")
		sinfo.ContractType = proto.String(kind)
		sinfo.Timestamp = proto.Uint64(tt)
		pb.Sinfo = sinfo
		okexQuoteReply(protocol.FID_QUOTE_KLine, pb)
	}
	return nil
}

/*
okex depth 的data元素的数据格式如下：
"data": {
         "asks": [
            ["59.522", "1.0", "0.168", "93.2443", "555.0"]
         ],
         "bids": [
            ["59.211", "500.0", "84.4437", "1364.2625", "8092.0"]
         ],
         "timestamp": 1505358555123
	  }
timestamp(long): 服务器时间戳
asks(array):卖单深度 数组索引(string) [价格, 量(张), 量(币),累计量(币),累积量(张)]
bids(array):买单深度 数组索引(string) [价格, 量(张), 量(币),累计量(币),累积量(张)]
*/
func parseNtfDepth(symbol string, kind string, ch string, js *simplejson.Json) error {
	asks := js.Get("asks")
	bids := js.Get("bids")
	pb := &protocol.PBFutureDepth{}

	var items []*protocol.PBFutureOBItem
	var err error
	if err, items = parseNtfDepthImpl(asks); err != nil {
		return err
	}
	pb.Asks = items
	if err, items = parseNtfDepthImpl(bids); err != nil {
		return err
	}
	pb.Bids = items
	tt := uint64(js.Get("timestamp").MustInt64())

	sinfo := &protocol.PBQuoteSymbol{}
	sinfo.Exchange = proto.String("okex")
	sinfo.Symbol = proto.String(symbol + "_usd")
	sinfo.ContractType = proto.String(kind)
	sinfo.Timestamp = proto.Uint64(tt)
	pb.Sinfo = sinfo
	okexQuoteReply(protocol.FID_QUOTE_Depth, pb)
	return nil
}

func parseNtfDepthImpl(js *simplejson.Json) (error, []*protocol.PBFutureOBItem) {
	arr, err := js.Array()
	if err != nil {
		return err, nil
	}
	ll := len(arr)
	items := make([]*protocol.PBFutureOBItem, 0, 10)
	for i := 0; i < ll; i++ {
		sub := js.GetIndex(i)
		subarr, err := sub.Array()
		if err != nil {
			return err, nil
		}
		if len(subarr) != 5 {
			logs.Error("parseNtfDepthImpl array num error, len[%d]", len(subarr))
			return errors.New("parseNtfDepthImpl array num error"), nil
		}

		price, _ := strconv.ParseFloat(sub.GetIndex(0).MustString(), 64)
		vol, _ := strconv.ParseFloat(sub.GetIndex(2).MustString(), 64)
		obi := &protocol.PBFutureOBItem{}
		obi.Price = proto.Float32(float32(price))
		obi.Vol = proto.Float32(float32(vol))
		items = append(items, obi)
	}
	return nil, items
}

/*
 okex 逐笔数据的data元素格式如下：
 "data":[["1093251376","3703.88","22.0","18:48:49","ask"],["1093251378","3703.88","28.0","18:48:49","ask"]]
 [交易序号, 价格, 成交量(张), 时间, 买卖类型]
 [string, string, string, string, string]
*/
func parseNtfTrade(symbol string, kind string, ch string, js *simplejson.Json) error {
	arr, err := js.Array()
	if err != nil {
		return err
	}
	ll := len(arr)
	for i := 0; i < ll; i++ {
		sub := js.GetIndex(i)
		subarr, err := sub.Array()
		if err != nil {
			return err
		}
		if len(subarr) != 5 {
			logs.Error("parseNtfTrade array num error, len[%d]", len(subarr))
			return errors.New("parseNtfTrade array num error")
		}

		pb := &protocol.PBFutureTrade{}
		pb.TradeSeq = proto.String(sub.GetIndex(0).MustString())

		price, _ := strconv.ParseFloat(sub.GetIndex(1).MustString(), 64)
		pb.Price = proto.Float32(float32(price))

		// 这里的是合约张数
		amount, _ := strconv.ParseFloat(sub.GetIndex(2).MustString(), 64)
		pb.Amount = proto.Int32(int32(amount))

		// 这里的时间是hour:minue:second，不符合要求
		// 所以只能采取本地时间补上年月日
		tm := time.Now()
		tstr := fmt.Sprintf("%04d-%02d-%02d ", tm.Year(), tm.Month(), tm.Day())
		tstr += sub.GetIndex(3).MustString()
		tm2, err := time.Parse(protocol.TM_LAYOUT_STR, tstr)
		tt := uint64(tm2.Unix() * 1000)
		if err != nil {
			logs.Error("ntf trade time error, tstr:%s. use local time", tstr)
			tt = uint64(time.Now().Unix() * 1000)
		}

		bs := sub.GetIndex(4).MustString()
		if bs == "ask" {
			pb.BsCode = proto.String("s")
		} else {
			pb.BsCode = proto.String("b")
		}

		sinfo := &protocol.PBQuoteSymbol{}
		sinfo.Exchange = proto.String("okex")
		sinfo.Symbol = proto.String(symbol + "_usd")
		sinfo.ContractType = proto.String(kind)
		sinfo.Timestamp = proto.Uint64(tt)
		pb.Sinfo = sinfo
		okexQuoteReply(protocol.FID_QUOTE_Trade, pb)
	}

	return nil
}

func okexQuoteReply(tid int, pb proto.Message) error {
	return utils.PackAndReplyToBroker(protocol.TOPIC_OKEX_QUOTE_PUB, "okex", tid, 0, pb)
}
