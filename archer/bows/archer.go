package bows

type ArcherCmd struct {
	Cmd          int
	ReqSerial    int
	Exchange     string
	Symbol       string
	ContractType string
	OrderType    int
	PriceSt      int
	Price        float32
	Amount       int
	Vol          float32
	Level        int
	OrderIDs     string
	TransType    int
	OrderStatus  int
	CurrentPage  int
	PageLength   int
}

type Archer interface {
	Init() error
	Run(chan *ArcherCmd)
}

const INTERNAL_CMD_EXIT = -877
