package bows

type bitfinexArcher struct {
	wsurl string
}

func newBitfinexArcher() Archer {
	return &bitfinexArcher{
		wsurl: "wss://api.bitfinex.com/ws",
	}
}

func (t *bitfinexArcher) Init() error {
	return nil
}

func (t *bitfinexArcher) Run(chan *ArcherCmd) {

}
