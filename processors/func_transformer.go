package processors

import (
	"github.com/dailyburn/ratchet/data"
)

// FuncTransformer executes the given function on each data
// payload, sending the resuling data to the next stage.
//
// While FuncTransformer is useful for simple data transformation, more
// complicated tasks justify building a custom implementation of DataProcessor.
type FuncTransformer struct {
	transform        func(d data.Payload) data.Payload
	Name             string // can be set for more useful log output
	ConcurrencyLevel int    // See ConcurrentDataProcessor
}

func NewFuncTransformer(transform func(d data.Payload) data.Payload) *FuncTransformer {
	return &FuncTransformer{transform: transform}
}

func (t *FuncTransformer) ProcessData(d data.Payload, outputChan chan data.Payload, killChan chan error) {
	outputChan <- t.transform(d)
}

func (t *FuncTransformer) Finish(outputChan chan data.Payload, killChan chan error) {
}

func (t *FuncTransformer) String() string {
	if t.Name != "" {
		return t.Name
	}
	return "FuncTransformer"
}

// See ConcurrentSee ConcurrentDataProcessor
func (t *FuncTransformer) Concurrency() int {
	return t.ConcurrencyLevel
}
