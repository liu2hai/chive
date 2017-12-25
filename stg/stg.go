package stg

import (
	"fmt"
	"time"

	"github.com/Shopify/sarama"
	"github.com/liu2hai/chive/config"
	"github.com/liu2hai/chive/kfc"
	"github.com/liu2hai/chive/logs"
	"github.com/liu2hai/chive/utils"
	"github.com/syndtr/goleveldb/leveldb"
)

/*
 path的格式为：/usr/slash/data/
 data下是各个交易所名称，交易所下面是日期
 /usr/slash/data/okex/2017-12-04/quote
*/

const (
	STG_CMD_EXIT              = 1
	STG_CMD_SWITCH_TRADINGDAY = 2
)

type storage struct {
	path       string
	tradingDay string
	dbm        map[string]*leveldb.DB
	currm      map[string]uint64
}

var stgt *storage
var dbName string
var countKey []byte

func init() {
	stgt = &storage{
		dbm:   make(map[string]*leveldb.DB),
		currm: make(map[string]uint64),
	}
	dbName = "quote"
	countKey = []byte("-1")
}

func StartStorage(ch chan int) error {
	stgt.tradingDay = getCurrDate()
	stgt.path = config.T.StgPath

	for _, exchange := range config.T.Exchanges {
		filename := makeDBFileName(stgt.path, exchange, stgt.tradingDay)

		db, err := leveldb.OpenFile(filename, nil)
		if err != nil {
			logs.Error("open leveldb file error [%s]", err.Error())
			return err
		}
		stgt.dbm[exchange] = db
		curr, err := db.Get(countKey, nil)
		if err != nil {
			stgt.currm[exchange] = 0
			db.Put(countKey, utils.UintTobytes(0), nil)
		} else {
			stgt.currm[exchange] = utils.BytesToUint(curr)
			logs.Info("open leveldb [%s], has %d records", filename, stgt.currm[exchange])
		}
	}

	go stgLoop(ch)
	return nil
}

func getCurrDate() string {
	t := time.Now()
	return fmt.Sprintf("%04d-%02d-%02d", t.Year(), t.Month(), t.Day())
}

func makeDBFileName(path string, exchange string, tradingDay string) string {
	name := path + exchange + "/" + tradingDay + "/" + dbName
	return name
}

func stgLoop(ch chan int) {
	defer doStgExit()

	for {
		select {
		case msg := <-kfc.ReadMessages():
			handleStgMsg(msg)

		case cmd, ok := <-ch:
			if !ok || cmd == STG_CMD_EXIT {
				return
			}
			if cmd == STG_CMD_SWITCH_TRADINGDAY {
				b := switchTradingDay()
				if !b {
					return
				}
			}
		}
	}
}

func doStgExit() {
	for _, v := range stgt.dbm {
		v.Close()
	}
}

func switchTradingDay() bool {
	newTradingDay := getCurrDate()
	if newTradingDay <= stgt.tradingDay {
		return true
	}

	oldTradingDay := stgt.tradingDay
	stgt.tradingDay = newTradingDay
	keys := []string{}
	for k, v := range stgt.dbm {
		v.Close()
		keys = append(keys, k)
	}

	for _, key := range keys {
		filename := makeDBFileName(stgt.path, key, stgt.tradingDay)
		db, err := leveldb.OpenFile(filename, nil)
		if err != nil {
			logs.Error("stg switch tradingday, openfile error: %s", err.Error())
			return false
		}
		db.Put(countKey, utils.UintTobytes(0), nil)
		stgt.dbm[key] = db
		stgt.currm[key] = 0

		logs.Info("exchange [%s] has switch tradingDay [%s] -> [%s]", key, oldTradingDay, newTradingDay)
	}

	return true
}

func handleStgMsg(msg *sarama.ConsumerMessage) bool {
	msgKey := string(msg.Key)
	db, ok := stgt.dbm[msgKey]
	if !ok {
		logs.Error("stg not supported msg, key: %s", msgKey)
		return false
	}

	key := utils.UintTobytes(stgt.currm[msgKey])
	err := db.Put(key, msg.Value, nil)
	if err != nil {
		filename := makeDBFileName(stgt.path, msgKey, stgt.tradingDay)
		logs.Error("stg write [%s] error [%s]", filename, err.Error())
		return false
	}

	stgt.currm[msgKey] += 1
	db.Put(countKey, utils.UintTobytes(stgt.currm[msgKey]), nil)
	return true
}
