package ratchet

import (
	"container/list"
	"fmt"
	"sync"

	"github.com/plandem/ratchet/data"
)

// DataProcessor is the interface that should be implemented to perform data-related
// tasks within a Pipeline. DataProcessors are responsible for receiving, processing,
// and then sending data on to the next stage of processing.
type DataProcessor interface {
	// ProcessData will be called for each data sent from the previous stage.
	// ProcessData is called with a data.Payload instance, which is the data being received,
	// an outputChan, which is the channel to send data to, and a killChan,
	// which is a channel to send unexpected errors to (halting execution of the Pipeline).
	ProcessData(d data.Payload, outputChan chan data.Payload, killChan chan error)

	// Finish will be called after the previous stage has finished sending data,
	// and no more data will be received by this DataProcessor. Often times
	// Finish can be an empty function implementation, but sometimes it is
	// necessary to perform final data processing.
	Finish(outputChan chan data.Payload, killChan chan error)
}

// dataProcessor is a type used internally to the Pipeline management
// code, and wraps a DataProcessor instance. DataProcessor is the main
// interface that should be implemented to perform work within the data
// pipeline, and this dataProcessor type simply embeds it and adds some
// helpful channel management and other attributes.
type dataProcessor struct {
	DataProcessor
	executionStat
	concurrentDataProcessor
	chanBrancher
	chanMerger
	outputs    []DataProcessor
	inputChan  chan data.Payload
	outputChan chan data.Payload
}

type chanBrancher struct {
	branchOutChans []chan data.Payload
}

func (dp *dataProcessor) branchOut(clone data.PayloadClone) {
	go func() {
		for d := range dp.outputChan {
			for _, out := range dp.branchOutChans {
				// Make a copy to ensure concurrent stages
				// can alter data as needed.
				out <- clone(d)
			}
			dp.recordDataSent(d)
		}
		// Once all data is received, also close all the outputs
		for _, out := range dp.branchOutChans {
			close(out)
		}
	}()
}

type chanMerger struct {
	mergeInChans []chan data.Payload
	mergeWait    sync.WaitGroup
}

func (dp *dataProcessor) mergeIn() {
	// Start a merge goroutine for each input channel.
	mergeData := func(c chan data.Payload) {
		for d := range c {
			dp.inputChan <- d
		}
		dp.mergeWait.Done()
	}
	dp.mergeWait.Add(len(dp.mergeInChans))
	for _, in := range dp.mergeInChans {
		go mergeData(in)
	}

	go func() {
		dp.mergeWait.Wait()
		close(dp.inputChan)
	}()
}

// Do takes a DataProcessor instance and returns the dataProcessor
// type that will wrap it for internal ratchet processing. The details
// of the dataProcessor wrapper type are abstracted away from the
// implementing end-user code. The "Do" function is named
// succinctly to provide a nicer syntax when creating a PipelineLayout.
// See the ratchet package documentation for code examples of creating
// a new branching pipeline layout.
func Do(processor DataProcessor) *dataProcessor {
	dp := dataProcessor{DataProcessor: processor}
	dp.outputChan = make(chan data.Payload)
	dp.inputChan = make(chan data.Payload)

	if isConcurrent(processor) {
		dp.concurrency = processor.(ConcurrentDataProcessor).Concurrency()
		dp.workThrottle = make(chan workSignal, dp.concurrency)
		dp.workList = list.New()
		dp.doneChan = make(chan bool)
		dp.inputClosed = false
	}

	return &dp
}

// Outputs should be called to specify which DataProcessor instances the current
// processor should send it's output to. See the ratchet package
// documentation for code examples and diagrams.
func (dp *dataProcessor) Outputs(processors ...DataProcessor) *dataProcessor {
	dp.outputs = processors
	return dp
}

// pass through String output to the DataProcessor
func (dp *dataProcessor) String() string {
	return fmt.Sprintf("%v", dp.DataProcessor)
}
