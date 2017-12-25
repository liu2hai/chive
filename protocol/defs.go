package protocol

const TM_LAYOUT_STR = "2006-01-02 15:04:05"

const (
	CMD_QRY_ACCOUNT    = 1
	CMD_QRY_POSITION   = 2
	CMD_SET_ORDER      = 3
	CMD_QRY_ORDERS     = 4
	CMD_CANCEL_ORDER   = 5
	CMD_TRANSFER_MONEY = 6
)

const (
	ORDERTYPE_OPENLONG   = 1 // 开多
	ORDERTYPE_OPENSHORT  = 2 // 开空
	ORDERTYPE_CLOSELONG  = 3 // 平多
	ORDERTYPE_CLOSESHORT = 4 // 平空
)

const (
	PRICE_ST_MARKET = 1 // 市价
	PRICE_ST_LIMIT  = 2 // 限价
)

const (
	TRANS_SPOT_TO_FUTURE = 1 // 将资金从现货账户转到合约账户
	TRANS_FUTURE_TO_SPOT = 2 // 将资金从合约账户转到现货账户
)

const (
	ORDERSTATUS_WAITTING  = 0  // 等待成交
	ORDERSTATUS_PARTDONE  = 1  // 部分成交
	ORDERSTATUS_COMPLETE  = 2  // 全部成交
	ORDERSTATUS_CANCELED  = -1 // 已撤
	ORDERSTATUS_CANCELING = 4  // 撤单处理中
)

const (
	TOPIC_OKEX_QUOTE_PUB  = "okex_quote_pub"
	TOPIC_OKEX_ARCHER_REQ = "okex_archer_req"
	TOPIC_OKEX_ARCHER_RSP = "okex_archer_rsp"
)

const (
	KL1Min  int32 = 1
	KL3Min  int32 = 2
	KL5Min  int32 = 3
	KL15Min int32 = 4
	KL30Min int32 = 5
	KL1H    int32 = 6
	KL1D    int32 = 7
)
