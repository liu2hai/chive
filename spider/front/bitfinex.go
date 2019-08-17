package front

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"chive/logs"
	"chive/protocol"
	"chive/utils"

	simplejson "github.com/bitly/go-simplejson"
	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
)

type bitfinexQuoter struct {
	wsurl   string
	pairs   []string
	chanIds map[int]string
}

func newBitfinexQuoter() ExchangeQuote {
	return &bitfinexQuoter{
		wsurl: "wss://api.bitfinex.com/ws",
		//		pairs: []string{"BTCUSD", "LTCUSD"},
		pairs:   []string{"LTCUSD"},
		chanIds: make(map[int]string),
	}
}

func (t *bitfinexQuoter) Init() error {
	return nil
}

func (t *bitfinexQuoter) Run() {
	for _, s := range t.pairs {
		go t.RunImpl(s)
	}
}

/*
  主协程开出读写2个协程，并监控他们是否退出，只要有一个退出
  主协程会结束链接，这2个协程遇到链接结束肯定会退出，主协程重新来过
*/
func (t *bitfinexQuoter) RunImpl(pair string) {
	for {
		c := utils.Reconnect(t.wsurl, "bitfinex", "quote")
		rgc := make(chan int)
		wgc := make(chan int)

		go bitfinexReadLoop(t, c, rgc, pair)
		go bitfinexWriteLoop(t, c, wgc, pair)

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
		logs.Error("bitfinex run %s restart.... ", pair)
	}
}

func bitfinexReadLoop(t *bitfinexQuoter, c *websocket.Conn, rgc chan int, pair string) {
	defer close(rgc)
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			logs.Error("bitfinex %s sub ws error read:%s", pair, err.Error())
			return
		}
		fmt.Println("ws收到通知：")
		fmt.Printf("%s\n", message)

		// 去除心跳回应
		if len(message) == len(`{"event":"pong"}`) {
			continue
		}

		js, err := simplejson.NewJson(message)
		if err != nil {
			logs.Error("%s sub ws parse json error:%s, json: %s", pair, err.Error(), message)
			return
		}

		err = handleBitfinexQuote(t, js, pair)
		if err != nil {
			return
		}
	}
}

func bitfinexWriteLoop(t *bitfinexQuoter, c *websocket.Conn, wgc chan int, pair string) {
	tc := time.NewTimer(hbInterval * time.Second)
	defer tc.Stop()
	defer close(wgc)

	bitfinexSubQuote(c, pair)

	for {
		data := bitfinexMakeHBPket()
		err := c.WriteMessage(websocket.TextMessage, []byte(data))
		if err != nil {
			logs.Error("bitfinex write goroutine write error, %s", err.Error())
			return
		}
		<-tc.C
		tc.Reset(hbInterval * time.Second)
	}
}

func bitfinexMakeHBPket() string {
	return `{"event":"ping"}`
}

func bitfinexSubQuote(c *websocket.Conn, pair string) {

	// tiker 数据
	tikerStr := "{\"event\":\"subscribe\",\"channel\":\"ticker\",\"pair\":\"%s\"}"
	tikerReq := fmt.Sprintf(tikerStr, pair)

	c.WriteMessage(websocket.TextMessage, []byte(tikerReq))

	// trade 数据
	tradeStr := "{\"event\":\"subscribe\",\"channel\":\"trades\",\"pair\":\"%s\"}"
	tradeReq := fmt.Sprintf(tradeStr, pair)

	c.WriteMessage(websocket.TextMessage, []byte(tradeReq))
}

////////////////////////////////////////////////////////////////////////////////////

func handleBitfinexQuote(t *bitfinexQuoter, js *simplejson.Json, pair string) error {
	_, err := js.Map()
	if err == nil {
		return handleBitfinexReply(t, js, pair)
	}

	arr, err := js.Array()
	if err == nil {
		ll := len(arr)
		return handleBitfinexQData(t, js, pair, ll)
	}
	return err
}

/*
bitfinex连接回应：成功 -- {"event":"info","version":1.1}； 失败--{"event":"info","code":"xxx", "msg":"xxx"}
心跳回应：{"event":"pong"}
订阅回应：成功--{"event":"subscribed","channel":"ticker","chanId":17,"pair":"LTCUSD"}
*/
func handleBitfinexReply(t *bitfinexQuoter, js *simplejson.Json, pair string) error {
	ev := js.Get("event").MustString()
	if ev == "info" {
		_, ok := js.CheckGet("version")
		if !ok {
			code := js.Get("code").MustString()
			msg := js.Get("msg").MustString()
			logs.Error("bitfinex ws quote exception, %s-%s", code, msg)
			return errors.New("bitfinex ws quote exception")
		}
		return nil
	}

	if ev == "pong" {
		return nil
	}

	if ev == "subscribed" {
		pp := js.Get("pair").MustString()
		ch := js.Get("channel").MustString()
		chId := js.Get("chanId").MustInt()
		if pp != pair {
			logs.Error("bitfinex sub not fit ")
			return errors.New("bitfinex ws quote exception")
		}
		t.chanIds[chId] = ch
		return nil
	}
	return errors.New("bitfinex ws quote exception")
}

/*
保持心跳包：[17,"hb"]
其他数据包: [chanId, [...]]
*/
func handleBitfinexQData(t *bitfinexQuoter, js *simplejson.Json, pair string, ll int) error {
	if ll == 2 {
		return nil
	}
	chId := js.GetIndex(0).MustInt()
	ch, ok := t.chanIds[chId]
	if ok {
		if ch == "ticker" {
			return handleBitfinexTicker(t, js, pair, ll)
		} else if ch == "trades" {
			return handleBitfinexTrades(t, js, pair, ll)
		}
	}
	return nil
}

/*
ticker: [17,45.682,2239.49728109,45.688,497.06116578,-6.452,-0.1237,45.688,733554.74929387,52.704,44.508]
BID	        float	Price of last highest bid
BID_SIZE	float	Size of the last highest bid
ASK	        float	Price of last lowest ask
ASK_SIZE	float	Size of the last lowest ask
DAILY_CHANGE	    float	Amount that the last price has changed since yesterday
DAILY_CHANGE_PERC	float	Amount that the price has changed expressed in percentage terms
LAST_PRICE	        float	Price of the last trade.
VOLUME	            float	Daily volume
HIGH	            float	Daily high
LOW	                float	Daily low
*/
func handleBitfinexTicker(t *bitfinexQuoter, js *simplejson.Json, pair string, ll int) error {
	if ll != 11 {
		logs.Error("bitfinex %s ticker format error", pair)
		return errors.New("bitfinex ticker format error")
	}
	pb := &protocol.PBFutureTick{}

	pb.Bid = proto.Float32(float32(js.GetIndex(1).MustFloat64()))
	pb.BidVol = proto.Float32(float32(js.GetIndex(2).MustFloat64()))
	pb.Ask = proto.Float32(float32(js.GetIndex(3).MustFloat64()))
	pb.AskVol = proto.Float32(float32(js.GetIndex(4).MustFloat64()))
	//dailyChange := proto.Float32(float32(js.GetIndex(5).MustFloat64()))
	//dailyChangePerc := proto.Float32(float32(js.GetIndex(6).MustFloat64()))
	pb.Last = proto.Float32(float32(js.GetIndex(7).MustFloat64()))
	pb.Vol = proto.Float32(float32(js.GetIndex(8).MustFloat64()))
	pb.High = proto.Float32(float32(js.GetIndex(9).MustFloat64()))
	pb.Low = proto.Float32(float32(js.GetIndex(10).MustFloat64()))

	fmt.Println("pb ticker string:")
	fmt.Println(pb.String())
	return nil
}

/*
trade:
[18,"te","4256771-LTCUSD",1506078727,45.688,0.365899]
[18, 'tu', '1234-BTCUSD', 15254529, 1443659698, 236.42, 0.49064538 ]
SEQ	      string	Trade sequence id
ID	      int	Trade database id
TIMESTAMP	int	Unix timestamp of the trade.
PRICE	  float	Price at which the trade was executed
±AMOUNT	  float	How much was bought (positive) or sold (negative).

tu是te的更详细的版本
*/
func handleBitfinexTrades(t *bitfinexQuoter, js *simplejson.Json, pair string, ll int) error {
	if ll != 7 {
		return nil
	}
	pb := &protocol.PBFutureTrade{}
	pb.TradeSeq = proto.String(strconv.FormatInt(js.GetIndex(3).MustInt64(), 10))
	pb.Price = proto.Float32(float32(js.GetIndex(5).MustFloat64()))
	vol := js.GetIndex(6).MustFloat64()
	pb.Vol = proto.Float32(float32(math.Abs(vol)))
	if vol > 0 {
		pb.BsCode = proto.String("b")
	} else {
		pb.BsCode = proto.String("s")
	}

	fmt.Println("pb trade string:")
	fmt.Println(pb.String())
	return nil
}
