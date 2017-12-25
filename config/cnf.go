package config

import (
	"fmt"
)

// app config
type AppCnf struct {
	AppID     int
	LogPath   string
	CnfPath   string
	Broker    string
	StgPath   string
	Exchanges []string

	Archer struct {
		Keys []ArcherKeys
	}

	InfluxDB struct {
		Addr string
	}

	Replay struct {
		Days []string
	}
}

type ArcherKeys struct {
	Apikey    string
	Secretkey string
}

var T *AppCnf

func (c *AppCnf) LoadConfig(cnfPath string) (err error) {
	cnf, err := NewConfig("json", cnfPath)
	if err != nil {
		return err
	}
	c.CnfPath = cnfPath
	c.LogPath = cnf.String("log")
	c.Broker = cnf.String("kafka::broker")
	c.StgPath = cnf.String("stg")
	c.Exchanges = cnf.Strings("exchanges")

	for _, e := range c.Exchanges {
		sk1 := fmt.Sprintf("archer::%s::apikey", e)
		sk2 := fmt.Sprintf("archer::%s::secretkey", e)
		k := ArcherKeys{
			Apikey:    cnf.String(sk1),
			Secretkey: cnf.String(sk2),
		}
		c.Archer.Keys = append(c.Archer.Keys, k)
	}

	c.InfluxDB.Addr = cnf.String("influxDB::addr")
	c.Replay.Days = cnf.Strings("replay::days")
	return err
}

func newAppCnf() *AppCnf {
	return &AppCnf{
		AppID: 1,
	}
}

func init() {
	T = newAppCnf()
}
