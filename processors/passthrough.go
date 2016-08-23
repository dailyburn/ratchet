package processors

import "github.com/dailyburn/ratchet/data"

// Passthrough simply passes the data on to the next stage.
type Passthrough struct {
	i int // setting a placeholder field - if we leave this as an empty struct we get some properties for comparison and memory addressing that are not desirable and cause comparison bugs (see: http://dave.cheney.net/2014/03/25/the-empty-struct)
}

func NewPassthrough() *Passthrough {
	return &Passthrough{}
}

func (r *Passthrough) ProcessData(d data.Payload, outputChan chan data.Payload, killChan chan error) {
	outputChan <- d
}

func (r *Passthrough) Finish(outputChan chan data.Payload, killChan chan error) {
}

func (r *Passthrough) String() string {
	return "Passthrough"
}
