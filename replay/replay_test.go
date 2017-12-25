package replay

import (
	"fmt"
	"testing"
)

func TestReplay(t *testing.T) {
	r := NewReplay()
	exchanges := []string{"../build/data/okex"}
	days := []string{"2017-12-24"}
	dirs, exs := makeupReplayDirs(exchanges, days)

	fmt.Println("dirs :", dirs)

	err := openFiles(dirs, exs, r)
	if err != nil {
		return
	}

	ch := make(chan int)
	go readLoop(r, ch)

	go func() {
		for {
			<-r.ReadMessages()
		}
	}()
	<-ch
}
