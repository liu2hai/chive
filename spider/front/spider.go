package front

import "github.com/liu2hai/chive/logs"

/*
  交易所行情服务接口
*/
type ExchangeQuote interface {
	Init() error
	Run()
}

/*
  StartQuoters -- 启动行情拉取服务
*/
func StartQuoters(exchanges []string) error {
	for _, ex := range exchanges {
		q := createExchangeQuoter(ex)
		if q == nil {
			logs.Error("exchange [%s] is not supported now!")
			continue
		}

		if err := q.Init(); err != nil {
			logs.Error("exchange [%s] init fail, error:", ex, err.Error())
			return err
		}
		go q.Run()
	}
	return nil
}

func createExchangeQuoter(ex string) ExchangeQuote {
	if ex == "okex" {
		return newOKExQuoter()
	} else if ex == "bitfinex" {
		return newBitfinexQuoter()
	}
	return nil
}
