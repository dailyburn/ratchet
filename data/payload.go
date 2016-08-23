package data

import (
	"fmt"
	//"bytes"
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

func Objects(p Payload)([]map[string]interface{}, error) {
	var objects []map[string]interface{}

	// return if we have null instead of object(s).
	//if bytes.Equal(p, []byte("null")) {
	//	logger.Debug("Objects: received null. Expected object or objects. Skipping.")
	//	return objects, nil
	//}

	var v interface{}
	err := Unmarshal(p, &v)
	if err != nil {
		return nil, err
	}

	// check if we have a single object or a slice of objects
	switch vv := v.(type) {
	case []interface{}:
		for _, o := range vv {
			objects = append(objects, o.(map[string]interface{}))
		}
	case map[string]interface{}:
		objects = []map[string]interface{}{vv}
	case []map[string]interface{}:
		objects = vv
	default:
		err = fmt.Errorf("Objects: unsupported data type: %T", vv)
		return nil, err
	}

	return objects, nil

}
