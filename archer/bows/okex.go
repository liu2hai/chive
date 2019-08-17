/*
  查询类的请求如果是API返回失败，都没有返回给后台处理
  因为后台也无法处理，唯有继续发起查询
*/
package bows

import (
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"

	simplejson "github.com/bitly/go-simplejson"

	"chive/config"
	"chive/logs"
	"chive/protocol"
	"chive/utils"
)

type okexArcher struct {
	wsurl     string
	resturl   string
	errm      map[int]string
	apikey    string
	secretkey string
}

func newOkexArcher() Archer {
	return &okexArcher{
		wsurl:   "wss://real.okex.com:10440/websocket/okexapi",
		resturl: "https://www.okex.com/api/v1",
		errm:    make(map[int]string),
	}
}

func (t *okexArcher) Init() error {
	utils.InitOkexErrorMap(t.errm)
	var i = -1
	for idx, e := range config.T.Exchanges {
		if e == "okex" {
			i = idx
			break
		}
	}
	if i < 0 {
		return errors.New("okex not in config")
	}
	if i >= len(config.T.Archer.Keys) {
		return errors.New("okex not in config2")
	}
	t.apikey = config.T.Archer.Keys[i].Apikey
	t.secretkey = config.T.Archer.Keys[i].Secretkey
	return nil
}

func doOkexExit() {
	logs.Info("okex archer exit ")
}

/*
1. 使用okex的http接口
*/
func (t *okexArcher) Run(archerCh chan *ArcherCmd) {
	for {
		cmd := <-archerCh

		logs.Info("archer 收到命令[%d]", cmd.Cmd)
		switch cmd.Cmd {
		case INTERNAL_CMD_EXIT:
			doOkexExit()
			return

		case protocol.CMD_QRY_ACCOUNT:
			t.qryMoneyInfo(cmd)

		case protocol.CMD_QRY_POSITION:
			t.qryHoldDetail(cmd)

		case protocol.CMD_SET_ORDER:
			t.setOrder(cmd)

		case protocol.CMD_QRY_ORDERS:
			t.qryOrdersInfo(cmd)

		case protocol.CMD_CANCEL_ORDER:
			t.cancelOrders(cmd)

		case protocol.CMD_TRANSFER_MONEY:
			t.transferMoney(cmd)
		}

		logs.Info("archer 完成命令[%d]", cmd.Cmd)
	}
}

// 查询用户账户信息
func (t *okexArcher) qryMoneyInfo(cmd *ArcherCmd) {
	resource := "/future_userinfo_4fix.do?"
	params := map[string]string{
		"api_key": t.apikey,
	}
	params["sign"] = buildMySign(params, t.secretkey)
	eid, js := doHttpPost(t.resturl, resource, params)
	handleRspQryMoneyInfo(eid, js, cmd.ReqSerial)
}

func handleRspQryMoneyInfo(eid int, js *simplejson.Json, reqSerial int) {
	pb := &protocol.PBFRspQryMoneyInfo{}
	rsp := &protocol.RspInfo{}
	rsp.ErrorId = proto.Int(eid)
	pb.Rsp = rsp

	if eid != protocol.ErrId_OK {
		return
	}
	result := js.Get("result").MustBool()
	if !result {
		logs.Error("请求资金信息API返回失败")
		rsp.ErrorId = proto.Int(protocol.ErrId_ApiError)
		return
	}
	handleDetailMoneyInfo(js.Get("info").Get("btc"), pb, "btc_usd")
	handleDetailMoneyInfo(js.Get("info").Get("ltc"), pb, "ltc_usd")
	handleDetailMoneyInfo(js.Get("info").Get("bch"), pb, "bch_usd")
	handleDetailMoneyInfo(js.Get("info").Get("eth"), pb, "eth_usd")
	handleDetailMoneyInfo(js.Get("info").Get("etc"), pb, "etc_usd")

	okexArcherReply(protocol.FID_RspQryMoneyInfo, reqSerial, pb)
}

/*
{
   "result": true,
   "info": {
      "btc": {
         "balance": 0,
         "rights": 0,
         "contracts": []
      },
      "ltc": {
         "balance": 1.55436666,
         "rights": 1.56675117,
         "contracts": [
            {
               "contract_type": "this_week",
               "freeze": 0,
               "balance": 0.01240155,
               "contract_id": 20171201116,
               "available": 1.55436666,
               "profit": -0.00001857,
               "bond": 0.01238298,
               "unprofit": 0
            }
         ]
      }
   }
}
*/
func handleDetailMoneyInfo(js *simplejson.Json, pb *protocol.PBFRspQryMoneyInfo, symbol string) {
	moneyInfo := &protocol.PBFMoneyInfo{}
	moneyInfo.Symbol = []byte(symbol)
	moneyInfo.Balance = proto.Float32(float32(js.Get("balance").MustFloat64()))
	moneyInfo.Rights = proto.Float32(float32(js.Get("rights").MustFloat64()))

	contracts := js.Get("contracts")
	arr, err := contracts.Array()
	if err != nil {
		return
	}
	ll := len(arr)
	for i := 0; i < ll; i++ {
		sub := contracts.GetIndex(i)
		spb := &protocol.PBFContractMoneyInfo{}
		spb.ContractType = []byte(sub.Get("contract_type").MustString())
		spb.Freeze = proto.Float32(float32(sub.Get("freeze").MustFloat64()))
		spb.Balance = proto.Float32(float32(sub.Get("balance").MustFloat64()))
		cid := strconv.FormatUint(sub.Get("contract_id").MustUint64(), 10)
		spb.ContractId = []byte(cid)
		spb.Available = proto.Float32(float32(sub.Get("available").MustFloat64()))
		spb.Profit = proto.Float32(float32(sub.Get("profit").MustFloat64()))
		spb.Unprofit = proto.Float32(float32(sub.Get("unprofit").MustFloat64()))
		spb.Bond = proto.Float32(float32(sub.Get("bond").MustFloat64()))

		moneyInfo.Contracts = append(moneyInfo.Contracts, spb)
	}
	pb.MoneyInfos = append(pb.MoneyInfos, moneyInfo)
}

// 查询用户持仓明细
// type 为1表示查询全部
func (t *okexArcher) qryHoldDetail(cmd *ArcherCmd) {
	resource := "/future_position_4fix.do?"
	params := map[string]string{
		"symbol":        cmd.Symbol,
		"contract_type": cmd.ContractType,
		"api_key":       t.apikey,
		"type":          "1",
	}
	params["sign"] = buildMySign(params, t.secretkey)
	eid, js := doHttpPost(t.resturl, resource, params)
	handleRspHoldDetail(eid, js, cmd)
}

func handleRspHoldDetail(eid int, js *simplejson.Json, cmd *ArcherCmd) {
	pb := &protocol.PBFRspQryPosInfo{}
	rsp := &protocol.RspInfo{}
	rsp.ErrorId = proto.Int(eid)

	pb.Rsp = rsp
	pb.Exchange = []byte(cmd.Exchange)
	pb.Symbol = []byte(cmd.Symbol)
	pb.ContractType = []byte(cmd.ContractType)

	if eid != protocol.ErrId_OK {
		return
	}
	result := js.Get("result").MustBool()
	if !result {
		logs.Error("请求头寸信息API返回失败")
		rsp.ErrorId = proto.Int(protocol.ErrId_ApiError)
		return
	}

	holding := js.Get("holding")
	arr, err := holding.Array()
	if err != nil {
		return
	}
	ll := len(arr)
	for i := 0; i < ll; i++ {
		sub := holding.GetIndex(i)
		subp := &protocol.PBFContractPosInfo{}
		subp.BuyAmount = proto.Float32(float32(sub.Get("buy_amount").MustFloat64()))
		subp.BuyAvailable = proto.Float32(float32(sub.Get("buy_available").MustFloat64()))
		subp.BuyBond = proto.Float32(float32(sub.Get("buy_bond").MustFloat64()))

		f1, _ := strconv.ParseFloat(sub.Get("buy_flatprice").MustString(), 64)
		subp.BuyFlatprice = proto.Float32(float32(f1))

		subp.BuyPriceAvg = proto.Float32(float32(sub.Get("buy_price_avg").MustFloat64()))
		subp.BuyPriceCost = proto.Float32(float32(sub.Get("buy_price_cost").MustFloat64()))

		f2, _ := strconv.ParseFloat(sub.Get("buy_profit_lossratio").MustString(), 64)
		subp.BuyProfitLossratio = proto.Float32(float32(f2))

		cid := strconv.FormatUint(sub.Get("contract_id").MustUint64(), 10)
		subp.ContractId = []byte(cid)
		subp.ContractType = []byte(sub.Get("contract_type").MustString())

		// json里的时间戳是毫秒数
		sec := int64(sub.Get("create_date").MustUint64() / 1000)
		tm := time.Unix(sec, 0)
		subp.CreateDate = []byte(tm.Format(protocol.TM_LAYOUT_STR))
		subp.LeverRate = proto.Int32(int32(sub.Get("lever_rate").MustInt()))

		subp.SellAmount = proto.Float32(float32(sub.Get("sell_amount").MustFloat64()))
		subp.SellAvailable = proto.Float32(float32(sub.Get("sell_available").MustFloat64()))
		subp.SellBond = proto.Float32(float32(sub.Get("sell_bond").MustFloat64()))

		f4, _ := strconv.ParseFloat(sub.Get("sell_flatprice").MustString(), 64)
		subp.SellFlatprice = proto.Float32(float32(f4))

		subp.SellPriceAvg = proto.Float32(float32(sub.Get("sell_price_avg").MustFloat64()))
		subp.SellPriceCost = proto.Float32(float32(sub.Get("sell_price_cost").MustFloat64()))

		f5, _ := strconv.ParseFloat(sub.Get("sell_profit_lossratio").MustString(), 64)
		subp.SellProfitLossratio = proto.Float32(float32(f5))

		subp.Symbol = []byte(sub.Get("symbol").MustString())

		pb.PosInfos = append(pb.PosInfos, subp)
	}

	okexArcherReply(protocol.FID_RspQryPosInfo, cmd.ReqSerial, pb)
}

// 下单
func (t *okexArcher) setOrder(cmd *ArcherCmd) {
	resource := "/future_trade.do?"
	params := map[string]string{
		"symbol":        cmd.Symbol,
		"contract_type": cmd.ContractType,
		"api_key":       t.apikey,
		"price":         fmt.Sprintf("%.2f", cmd.Price),
		"amount":        fmt.Sprintf("%d", cmd.Amount),
		"type":          fmt.Sprintf("%d", cmd.OrderType),
		"match_price":   fmt.Sprintf("%d", cmd.PriceSt),
		"lever_rate":    fmt.Sprintf("%d", cmd.Level),
	}
	params["sign"] = buildMySign(params, t.secretkey)
	eid, js := doHttpPost(t.resturl, resource, params)
	handleRspSetOrderInfo(eid, js, t, cmd)
	logs.Info("okex下单，商品[%s], 合约类型[%s], 合约张数[%d], 订单类型[%s], 价格[%f], 杠杠[%d], reqSerial[%d]",
		cmd.Symbol, cmd.ContractType, cmd.Amount, utils.OrderTypeStr(int32(cmd.OrderType)), cmd.Price, cmd.Level, cmd.ReqSerial)
}

func handleRspSetOrderInfo(eid int, js *simplejson.Json, t *okexArcher, cmd *ArcherCmd) {
	pb := &protocol.PBFRspSetOrder{}
	rsp := &protocol.RspInfo{}
	rsp.ErrorId = proto.Int(eid)

	pb.Rsp = rsp
	pb.Exchange = []byte(cmd.Exchange)
	pb.Symbol = []byte(cmd.Symbol)
	pb.ContractType = []byte(cmd.ContractType)

	if eid != protocol.ErrId_OK {
		return
	}
	result := js.Get("result").MustBool()
	if !result {
		msg := t.errm[js.Get("error_code").MustInt()]
		logs.Error("请求订单信息API返回失败, error [%s]", msg)
		rsp.ErrorId = proto.Int(protocol.ErrId_ApiError)
		rsp.ErrorMsg = []byte(msg)
	} else {
		pb.OrderId = []byte(strconv.FormatUint(js.Get("order_id").MustUint64(), 10))
	}
	okexArcherReply(protocol.FID_RspSetOrder, cmd.ReqSerial, pb)
}

func (t *okexArcher) qryOrdersByStatus(cmd *ArcherCmd) {
	resource := "/future_order_info.do?"
	params := map[string]string{
		"symbol":        cmd.Symbol,
		"contract_type": cmd.ContractType,
		"api_key":       t.apikey,
		"order_id":      "-1",
		"status":        fmt.Sprintf("%d", cmd.OrderStatus),
		"current_page":  fmt.Sprintf("%d", cmd.CurrentPage),
		"page_length":   fmt.Sprintf("%d", cmd.PageLength),
	}
	params["sign"] = buildMySign(params, t.secretkey)
	eid, js := doHttpPost(t.resturl, resource, params)
	handleRspQryOrdersInfo(eid, js, t, cmd)
}

func (t *okexArcher) qryOrdersById(cmd *ArcherCmd) {
	resource := "/future_orders_info.do?"
	params := map[string]string{
		"symbol":        cmd.Symbol,
		"contract_type": cmd.ContractType,
		"api_key":       t.apikey,
		"order_id":      cmd.OrderIDs,
	}
	params["sign"] = buildMySign(params, t.secretkey)
	eid, js := doHttpPost(t.resturl, resource, params)
	handleRspQryOrdersInfo(eid, js, t, cmd)
}

// 批量查询单据信息, order_id以,分割，一次最多查询50个
func (t *okexArcher) qryOrdersInfo(cmd *ArcherCmd) {
	if cmd.OrderIDs == "-1" {
		t.qryOrdersByStatus(cmd)
	} else {
		t.qryOrdersById(cmd)
	}
}

func handleRspQryOrdersInfo(eid int, js *simplejson.Json, t *okexArcher, cmd *ArcherCmd) {
	pb := &protocol.PBFRspQryOrders{}
	rsp := &protocol.RspInfo{}
	rsp.ErrorId = proto.Int(eid)
	pb.Rsp = rsp
	if eid != protocol.ErrId_OK {
		return
	}
	result := js.Get("result").MustBool()
	if !result {
		msg := t.errm[js.Get("error_code").MustInt()]
		logs.Error("请求查询订单信息API返回失败, error[%s]", msg)
		rsp.ErrorId = proto.Int(protocol.ErrId_ApiError)
		rsp.ErrorMsg = []byte(msg)
		return
	}
	orders := js.Get("orders")
	arr, err := orders.Array()
	if err != nil {
		return
	}
	ll := len(arr)
	for i := 0; i < ll; i++ {
		sub := orders.GetIndex(i)
		subp := &protocol.PBFOrderInfo{}
		parseOrderInfo(sub, subp)
		subp.ContractType = []byte(cmd.ContractType)
		pb.Orders = append(pb.Orders, subp)
	}

	okexArcherReply(protocol.FID_RspQryOrders, cmd.ReqSerial, pb)
}

func parseOrderInfo(js *simplejson.Json, pb *protocol.PBFOrderInfo) {
	pb.Amount = proto.Float32(float32(js.Get("amount").MustFloat64()))
	pb.ContractName = []byte(js.Get("contract_name").MustString())
	sec := int64(js.Get("create_date").MustUint64() / 1000)
	tm := time.Unix(sec, 0)
	pb.ContractDate = []byte(tm.Format(protocol.TM_LAYOUT_STR))
	pb.DealAmount = proto.Float32(float32(js.Get("deal_amount").MustFloat64()))
	pb.Fee = proto.Float32(float32(js.Get("fee").MustFloat64()))
	pb.LeverRate = proto.Int32(int32(js.Get("lever_rate").MustInt()))
	id := strconv.FormatUint(js.Get("order_id").MustUint64(), 10)
	pb.OrderId = []byte(id)
	pb.Price = proto.Float32(float32(js.Get("price").MustFloat64()))
	pb.PriceAvg = proto.Float32(float32(js.Get("price_avg").MustFloat64()))
	pb.Status = proto.Int32(int32(js.Get("status").MustInt()))
	pb.Symbol = []byte(js.Get("symbol").MustString())
	pb.Type = proto.Int32(int32(js.Get("type").MustInt()))
	pb.UnitAmount = proto.Float32(float32(js.Get("unit_amount").MustFloat64()))
}

// 撤销单据, order_id以,分割，一次最多撤销3个
func (t *okexArcher) cancelOrders(cmd *ArcherCmd) {
	resource := "/future_cancel.do?"
	params := map[string]string{
		"symbol":        cmd.Symbol,
		"contract_type": cmd.ContractType,
		"api_key":       t.apikey,
		"order_id":      cmd.OrderIDs,
	}
	params["sign"] = buildMySign(params, t.secretkey)
	eid, js := doHttpPost(t.resturl, resource, params)
	handleRspCancelOrdersInfo(eid, js, t, cmd)
	logs.Info("okex撤单, 商品[%s], 合约类型[%s], 订单号[%s]", cmd.Symbol, cmd.ContractType, cmd.OrderIDs)
}

func handleRspCancelOrdersInfo(eid int, js *simplejson.Json, t *okexArcher, cmd *ArcherCmd) {
	pb := &protocol.PBFRspCancelOrders{}
	rsp := &protocol.RspInfo{}
	rsp.ErrorId = proto.Int(eid)
	pb.Rsp = rsp
	if eid != protocol.ErrId_OK {
		return
	}
	// 如果是单笔返回，有result参数
	_, b := js.CheckGet("result")
	if !b {
		// 多笔返回
		s1 := strings.Split(js.Get("success").MustString(), ",")
		for _, v := range s1 {
			pb.Success = append(pb.Success, []byte(v))
		}
		s2 := strings.Split(js.Get("error").MustString(), ",")
		for _, v := range s2 {
			ids := strings.Split(v, ":")
			if len(ids) > 1 {
				pb.Errors = append(pb.Errors, []byte(ids[0]))
			}
		}
	} else {
		result := js.Get("result").MustBool()
		if result {
			// 单笔返回
			id := []byte(strconv.FormatUint(js.Get("order_id").MustUint64(), 10))
			pb.Success = append(pb.Success, id)

		} else {
			msg := t.errm[js.Get("error_code").MustInt()]
			logs.Error("撤销订单信息API返回失败, error [%s]", msg)
			rsp.ErrorId = proto.Int(protocol.ErrId_ApiError)
			rsp.ErrorMsg = []byte(msg)
		}
	}

	pb.Exchange = []byte(cmd.Exchange)
	pb.Symbol = []byte(cmd.Symbol)
	pb.ContractType = []byte(cmd.ContractType)
	okexArcherReply(protocol.FID_RspCancelOrders, cmd.ReqSerial, pb)
}

// 在现货和期货间划转资金
func (t *okexArcher) transferMoney(cmd *ArcherCmd) {
	resource := "/future_devolve.do?"
	params := map[string]string{
		"symbol":  cmd.Symbol,
		"api_key": t.apikey,
		"type":    fmt.Sprintf("%d", cmd.TransType),
		"amount":  fmt.Sprintf("%.6f", cmd.Vol),
	}
	params["sign"] = buildMySign(params, t.secretkey)
	eid, js := doHttpPost(t.resturl, resource, params)
	handleRspTransMoney(eid, js, t, cmd.ReqSerial)
	logs.Info("okex现期划转, 商品[%s], 币量[%f], 划转方向[%d]", cmd.Symbol, cmd.Vol, cmd.TransType)
}

func handleRspTransMoney(eid int, js *simplejson.Json, t *okexArcher, reqSerial int) {
	pb := &protocol.PBFRspTransferMoney{}
	rsp := &protocol.RspInfo{}
	rsp.ErrorId = proto.Int(eid)
	pb.Rsp = rsp
	if eid != protocol.ErrId_OK {
		return
	}
	result := js.Get("result").MustBool()
	if result {
		rsp.ErrorId = proto.Int32(protocol.ErrId_OK)
	} else {
		rsp.ErrorId = proto.Int32(protocol.ErrId_TransferErr)
		msg := t.errm[js.Get("error_code").MustInt()]
		rsp.ErrorMsg = []byte(msg)
		logs.Error("转账API返回失败, error [%s]", msg)
	}

	okexArcherReply(protocol.FID_RspTransferMoney, reqSerial, pb)
}

///////////////////////////////////////////////////////////

func buildMySign(params map[string]string, secretkey string) string {
	sign := ""
	keys := make([]string, len(params))
	var i = 0
	for k, _ := range params {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	for _, e := range keys {
		sign += e + "=" + params[e] + "&"
	}
	data := sign + "secret_key=" + secretkey
	data2 := md5.Sum([]byte(data))
	return strings.ToUpper(hex.EncodeToString(data2[:]))
}

func buildMyurl(resturl string, resource string, params map[string]string) string {
	url := resturl + resource
	if len(url) < 1 {
		return ""
	}
	for k, v := range params {
		url += k + "=" + v + "&"
	}
	bs := []byte(url)
	return string(bs[0 : len(bs)-1])
}

func buildMyBody(params map[string]string) []byte {
	str := ""
	for k, v := range params {
		str += k + "=" + v + "&"
	}
	if len(str) < 1 {
		return []byte{}
	}
	bs := []byte(str)
	return bs[0 : len(bs)-1]
}

func makeTLSClient(skipVerify bool) *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: skipVerify},
	}
	return &http.Client{Transport: tr}
}

func makeNormalClient() *http.Client {
	return &http.Client{}
}

func doHttpPost(resturl string, resource string, params map[string]string) (int, *simplejson.Json) {
	url := resturl + resource
	b := buildMyBody(params)
	fmt.Println("************************>")
	fmt.Println("url-body", url, string(b))

	client := makeNormalClient()
	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
	if err != nil {
		logs.Error("构建request出错")
		return protocol.ErrId_Internel, nil
	}
	req.Header.Set("Content-type", "application/x-www-form-urlencoded")
	rsp, err := client.Do(req)
	if err != nil {
		logs.Error("服务器无回应")
		return protocol.ErrId_ApiOutofService, nil
	}
	defer rsp.Body.Close()
	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		logs.Error("无法得到服务器回应的body")
		return protocol.ErrId_ApiError, nil
	}
	fmt.Println("服务器回应: ", rsp.StatusCode, string(body))
	if rsp.StatusCode != http.StatusOK {
		// 403	    用户请求过快，IP被屏蔽
		logs.Error("HTTP POST返回状态码错误[%d]", rsp.StatusCode)
		return protocol.ErrId_ApiError, nil
	}

	js, err := simplejson.NewJson(body)
	if err != nil {
		logs.Error("HTTP POST返回内容不是合法json: %s", string(body))
		return protocol.ErrId_ApiError, nil
	}
	return protocol.ErrId_OK, js
}

func okexArcherReply(tid int, reqSerial int, pb proto.Message) error {
	return utils.PackAndReplyToBroker(protocol.TOPIC_OKEX_ARCHER_RSP, "okex", tid, reqSerial, pb)
}
