package protocol

import (
	"bytes"
	"encoding/binary"
)

type Package interface {
	GetHeaderLen() int32
	ParseFromArray([]byte) bool
	SerialToArray() []byte
	GetBodyLen() uint32
	GetTid() uint32
	GetReqSerial() uint32
	GetAttribute() uint32
	GetPayload() []byte
}

/*
  package protocol
  length		int32
  tid			int32
  reqSerial		int32
  attribute		int16
  payload		[]byte

*/

type FixPackage struct {
	BodyLen   uint32
	Tid       uint32
	ReqSerial uint32
	Attribute uint32
	Payload   []byte
}

const FIX_PACKAGE_HEADERLEN = 16

func (t *FixPackage) GetHeaderLen() int32 {
	return FIX_PACKAGE_HEADERLEN
}

func (t *FixPackage) GetBodyLen() uint32 {
	return t.BodyLen
}

func (t *FixPackage) GetTid() uint32 {
	return t.Tid
}

func (t *FixPackage) GetReqSerial() uint32 {
	return t.ReqSerial
}

func (t *FixPackage) GetAttribute() uint32 {
	return t.Attribute
}

func (t *FixPackage) GetPayload() []byte {
	return t.Payload
}

func (t *FixPackage) ParseFromArray(data []byte) bool {
	ll := len(data)
	if ll < FIX_PACKAGE_HEADERLEN {
		return false
	}
	t.BodyLen = binary.BigEndian.Uint32(data[0:4])
	t.Tid = binary.BigEndian.Uint32(data[4:8])
	t.ReqSerial = binary.BigEndian.Uint32(data[8:12])
	t.Attribute = binary.BigEndian.Uint32(data[12:16])
	if uint32(ll-FIX_PACKAGE_HEADERLEN) < t.BodyLen {
		return false
	}
	t.Payload = data[16 : 16+t.BodyLen]
	return true
}

func (t *FixPackage) SerialToArray() []byte {
	t.BodyLen = uint32(len(t.Payload))
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, t.BodyLen)
	binary.Write(buf, binary.BigEndian, t.Tid)
	binary.Write(buf, binary.BigEndian, t.ReqSerial)
	binary.Write(buf, binary.BigEndian, t.Attribute)
	buf.Write(t.Payload)
	return buf.Bytes()
}
