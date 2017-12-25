package replay

import (
	"github.com/Shopify/sarama"
	"github.com/liu2hai/chive/config"
	"github.com/liu2hai/chive/logs"
	"github.com/liu2hai/chive/utils"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

var dbName = "quote"
var countKey []byte = []byte("-1")

const max_ch_len = 100

////////////////////////////////////////////////////////////////////////////////////////////////////

type Replay struct {
	msgq chan *sarama.ConsumerMessage
	dbm  map[string][]*leveldb.DB
}

func NewReplay() *Replay {
	return &Replay{
		msgq: make(chan *sarama.ConsumerMessage, max_ch_len),
		dbm:  make(map[string][]*leveldb.DB),
	}
}

func (r *Replay) ReadMessages() <-chan *sarama.ConsumerMessage {
	return r.msgq
}

////////////////////////////////////////////////////////////////////////////////////////////////////

func StartReplay(ch chan int) (*Replay, error) {
	r := NewReplay()
	exchanges := config.T.Exchanges
	days := config.T.Replay.Days
	dirs, exs := makeupReplayDirs(exchanges, days)
	err := openFiles(dirs, exs, r)
	if err != nil {
		return r, err
	}

	go readLoop(r, ch)
	return r, nil
}

// 根据配置找到要回放的目录
func makeupReplayDirs(exchanges []string, days []string) ([]string, []string) {
	ret := []string{}
	exs := []string{}
	dataDir := config.T.StgPath
	for _, ex := range exchanges {
		for _, d := range days {
			name := dataDir + ex + "/" + d + "/" + dbName
			ret = append(ret, name)
			exs = append(exs, ex)
		}
	}
	return ret, exs
}

// 打开每一个目录文件, 不存在会报错
func openFiles(dirs []string, exs []string, r *Replay) error {
	o := opt.Options{ErrorIfMissing: true}
	for i, filename := range dirs {
		db, err := leveldb.OpenFile(filename, &o)
		if err != nil {
			logs.Error("open leveldb file error [%s], file[%s]", err.Error(), filename)
			return err
		}
		key := exs[i]
		arr, ok := r.dbm[key]
		if !ok {
			arr = []*leveldb.DB{}
		}
		arr = append(arr, db)
		r.dbm[key] = arr
	}
	return nil
}

func readLoop(r *Replay, ch chan int) {
	defer doExit(r, ch)

	var c = 1
	var total = 0
	for _, arr := range r.dbm {
		total += len(arr)
	}

	for ex, arr := range r.dbm {
		for _, v := range arr {
			logs.Info("正在回放第[%d]个目录，共[%d]个目录", c, total)
			err := readOneFile(r, v, ex)
			if err != nil {
				return
			}
			logs.Info("第[%d]个目录回放完毕", c)
			c += 1
		}
	}
}

func doExit(r *Replay, ch chan int) {
	for _, arr := range r.dbm {
		for _, v := range arr {
			v.Close()
		}
	}
	close(ch)
	logs.Info("replay read loop exit...")
}

func readOneFile(r *Replay, db *leveldb.DB, ex string) error {
	tdata, err := db.Get(countKey, nil)
	if err != nil {
		logs.Error("读取countkey失败")
		return err
	}
	total := utils.BytesToUint(tdata)
	logs.Info("共有[%d]条记录", total)

	var i uint64
	for i = 0; i < total; i++ {
		val, err := db.Get(utils.UintTobytes(i), nil)
		if err != nil {
			logs.Error("读取[%d]条记录时失败", i)
			return err
		}

		msg := &sarama.ConsumerMessage{
			Key:   []byte(ex),
			Value: sarama.ByteEncoder(val),
		}
		r.msgq <- msg
	}
	return nil
}
