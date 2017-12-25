package protocol

const (

	// 行情--分笔
	FID_QUOTE_TICK = 1000

	// 行情--k线
	FID_QUOTE_KLine = 1001

	// 行情--档口
	FID_QUOTE_Depth = 1002

	// 行情--报价
	FID_QUOTE_Trade = 1003

	// 行情--指数
	FID_QUOTE_Index = 1004

	// 查询资金信息请求
	FID_ReqQryMoneyInfo = 2001

	// 查询资金信息回应
	FID_RspQryMoneyInfo = 2002

	// 查询头寸请求
	FID_ReqQryPosInfo = 2003

	// 查询头寸回应
	FID_RspQryPosInfo = 2004

	// 下单请求
	FID_ReqSetOrder = 2005

	// 下单回应
	FID_RspSetOrder = 2006

	// 批量查询单据请求
	FID_ReqQryOrders = 2007

	// 批量查询单据回应
	FID_RspQryOrders = 2008

	// 批量撤销单据请求
	FID_ReqCancelOrders = 2009

	// 批量撤销单据回应
	FID_RspCancelOrders = 2010

	// 在现货和合约账号划转资金请求
	FID_ReqTransferMoney = 2011

	// 在现货和合约账号划转资金回应
	FID_RspTransferMoney = 2012
)
