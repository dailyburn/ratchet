package data

// Payload is the data type that is passed along all data channels.
// Under the covers, Payload is simply a []byte containing binary data.
// It's up to you what serializer to use
//type Payload []byte
type Payload []byte

type PayloadClone func(Payload) (Payload)

