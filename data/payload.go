package data

import (
	"fmt"
	"github.com/dailyburn/ratchet/logger"
)

// Payload is the data type that is passed along all data channels.
// Under the covers, Payload is simply a []byte containing binary data.
// It's up to you what serializer to use
//type Payload []byte

type Payload interface {
	Clone() (Payload)
	marshal() []byte
	unmarshal(v interface{}) (error)
}

func Marshal(p Payload) []byte {
	return p.marshal()
}

func Unmarshal(p Payload, v interface{}) (error) {
	err := p.unmarshal(v)
	if err != nil {
		logger.Debug(fmt.Sprintf("data: failure to unmarshal payload into %+v - error is \"%v\"", v, err.Error()))
		logger.Debug(fmt.Sprintf("	Failed Data: %+v", p))
	}

	return err
}

func UnmarshalSilent(p Payload, v interface{}) (error) {
	return p.unmarshal(v)
}
