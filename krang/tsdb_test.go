package krang

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/liu2hai/chive/protocol"
)

func TestCreateAndShowDB(t *testing.T) {
	d := &influxdb{
		addr:   "http://localhost:8086", //config.T.InfluxDB.Addr,
		client: &http.Client{},
		antm:   make(map[string]*ant),
	}

	/*
		d.CreateDatabase("youdix")
		rets := d.ShowDatabase()
		fmt.Println(rets)
		d.testwrite()
	*/

	d.Check("okex", []string{"kkk"}, []string{"this_day", "this_month"})
	fmt.Println("d dbnames:", d.dbNames)

	d.Start()
	pb := &protocol.PBFutureTick{}

	tt := uint64(time.Now().Unix() * 1000)
	for i := 0; i < 160; i++ {
		pb.Vol = proto.Float32(0.2)
		pb.High = proto.Float32(156)
		pb.Low = proto.Float32(156)
		pb.DayVol = proto.Float32(156)
		pb.DayHigh = proto.Float32(156)
		pb.DayLow = proto.Float32(156)
		pb.Last = proto.Float32(156)
		pb.Bid = proto.Float32(156)
		pb.Ask = proto.Float32(float32(1 + i))
		pb.BidVol = proto.Float32(156)
		pb.AskVol = proto.Float32(156)
		pb.Sinfo = &protocol.PBQuoteSymbol{
			Exchange:     proto.String("okex"),
			Symbol:       proto.String("kkk"),
			ContractType: proto.String("this_day"),
			Timestamp:    proto.Uint64(tt),
		}

		d.StoreTick(pb)
	}
	<-time.After(2 * time.Second)
}
