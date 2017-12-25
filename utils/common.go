package utils

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/liu2hai/chive/kfc"
	"github.com/liu2hai/chive/protocol"
)

func PackAndReplyToBroker(topic string, key string, tid int, reqSerial int, pb proto.Message) error {
	data, err := proto.Marshal(pb)
	if err != nil {
		return err
	}

	pket := &protocol.FixPackage{}
	pket.BodyLen = uint32(len(data))
	pket.Tid = uint32(tid)
	pket.ReqSerial = uint32(reqSerial)
	pket.Attribute = 0
	pket.Payload = data

	bin := pket.SerialToArray()
	kfc.SendMessage(topic, key, bin)
	return nil
}

func MakeupSinfo(ex string, symbol string, contractType string) string {
	return ex + "_" + symbol + "_" + contractType
}

func UintTobytes(i uint64) []byte {
	return []byte(fmt.Sprintf("%d", i))
}

func BytesToUint(b []byte) uint64 {
	u, err := strconv.ParseUint(string(b), 10, 64)
	if err != nil {
		u = 0
	}
	return u
}

func OrderTypeStr(ot int32) string {
	switch ot {
	case protocol.ORDERTYPE_OPENLONG:
		return "开多"
	case protocol.ORDERTYPE_OPENSHORT:
		return "开空"
	case protocol.ORDERTYPE_CLOSELONG:
		return "平多"
	case protocol.ORDERTYPE_CLOSESHORT:
		return "平空"
	}
	return "未知"
}

func KLineStr(kl int32) string {
	switch kl {
	case protocol.KL1Min:
		return "KL1Min"
	case protocol.KL3Min:
		return "KL3Min"
	case protocol.KL5Min:
		return "KL5Min"
	case protocol.KL15Min:
		return "KL15Min"
	case protocol.KL30Min:
		return "KL30Min"
	case protocol.KL1H:
		return "KL1H"
	case protocol.KL1D:
		return "KL1D"
	}
	return "未知"
}

func TSStr(ts int64) string {
	return time.Unix(ts, 0).Format(protocol.TM_LAYOUT_STR)
}
