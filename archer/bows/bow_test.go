package bows

import (
	"fmt"
	"testing"
	"time"

	"github.com/liu2hai/chive/protocol"
)

func TestTm(t *testing.T) {
	tm := time.Now()
	fmt.Println(tm.Format(time.UnixDate))
	fmt.Println(tm.Format("2006-01-02 15:04:05"))

	tstr := fmt.Sprintf("%04d-%02d-%02d ", tm.Year(), tm.Month(), tm.Day())
	tstr += "18:48:49"
	fmt.Println("tstr: ", tstr)

	tm2, err := time.Parse(protocol.TM_LAYOUT_STR, tstr)
	tt := tm2.Unix() * 1000
	if err != nil {
		fmt.Println("ntf trade time error, tstr:%s. use local time", tstr)
		tt = int64(time.Now().Unix() * 1000)
		fmt.Println("here: ", tt)
	}
	fmt.Println("tt: ", tt)
}
