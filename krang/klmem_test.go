package krang

import (
	"fmt"
	"testing"

	"chive/protocol"

	"github.com/golang/protobuf/proto"
)

func TestKlmem(t *testing.T) {
	pbk5 := &protocol.PBFutureKLine{
		Kind: protocol.KL5Min,
	}
	pbk15 := &protocol.PBFutureKLine{
		Kind: protocol.KL15Min,
	}

	klm := NewKLineMem("test")
	for i := 0; i < 100; i++ {
		pbk5.Close = proto.Float32(float32(i + 1))
		pbk15.Close = proto.Float32(float32(i + 2))
		klm.addKLine(pbk5)
		klm.addKLine(pbk15)
	}

	fmt.Println("5min ma7:", klm.queryMA7(KL5MIN, 200))
	fmt.Println("5min ma15:", klm.queryMA15(KL5MIN, 50))

	fmt.Println("15min ma7:", klm.queryMA7(KL15MIN, 50))
	fmt.Println("15min ma15:", klm.queryMA15(KL15MIN, 50))
}
