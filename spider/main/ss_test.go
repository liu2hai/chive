package main

import (
	"fmt"
	"testing"

	"github.com/bitly/go-simplejson"
)

func Test_simplejs(t *testing.T) {
	ss := `[{"data":{"result":"false","error_code":"20104"},"channel":"ok_sub_futurusd_ltc_ticker_this_week"}]`
	js, err := simplejson.NewJson([]byte(ss))
	if err != nil {
		fmt.Println("error:", err)
	}
	arr, err := js.Array()
	if err != nil {
		fmt.Println("error:", err)
	}
	for i := 0; i < len(arr); i++ {
		fmt.Println("data:", arr[i])
	}
}
