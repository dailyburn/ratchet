package ratchet

import (
	"container/list"
	"sync"

	"github.com/plandem/ratchet/data"
	"github.com/dailyburn/ratchet/logger"
)

// ConcurrentDataProcessor is a DataProcessor that also defines
// a level of concurrency. For example, if Concurrency() returns 2,
// then the pipeline will allow the stage to execute up to 2 ProcessData()
// calls concurrently.
//
// Note that the order of data processing is maintained, meaning that
// when a DataProcessor receives ProcessData calls d1, d2, ..., the resulting data
// payloads sent on the outputChan will be sent in the same order as received.
type ConcurrentDataProcessor interface {
	DataProcessor
	Concurrency() int
}

// IsConcurrent returns true if the given DataProcessor implements ConcurrentDataProcessor
func isConcurrent(p DataProcessor) bool {
	_, ok := interface{}(p).(ConcurrentDataProcessor)
	return ok
}

// dataProcessor embeds concurrentDataProcessor
type concurrentDataProcessor struct {
	concurrency  int
	workThrottle chan workSignal
	workList     *list.List
	doneChan     chan bool
	inputClosed  bool
	sync.Mutex
}

type workSignal struct{}

type result struct {
	done       bool
	data       []data.Payload
	outputChan chan data.Payload
	open       bool
}

func (dp *dataProcessor) processData(d data.Payload, killChan chan error) {
	logger.Debug("dataProcessor: processData", dp, "with concurrency =", dp.concurrency)
	// If no concurrency is needed, simply call stage.ProcessData and return...
	if dp.concurrency <= 1 {
		dp.recordExecution(func() {
			dp.ProcessData(d, dp.outputChan, killChan)
		})
		return
	}
	// ... otherwise process the data in a concurrent queue/pool of goroutines
	logger.Debug("dataProcessor: processData", dp, "waiting for work")
	// wait for room in the queue
	dp.workThrottle <- workSignal{}
	logger.Debug("dataProcessor: processData", dp, "work obtained")
	rc := make(chan data.Payload)
	done := make(chan bool)
	exit := make(chan bool)
	// setup goroutine to handle result
	go func() {
		res := result{outputChan: dp.outputChan, data: []data.Payload{}, open: true}
		dp.Lock()
		dp.workList.PushBack(&res)
		dp.Unlock()
		logger.Debug("dataProcessor: processData", dp, "waiting to receive data on result chan")
		for {
			select {
			case d, open := <-rc:
				logger.Debug("dataProcessor: processData", dp, "received data on result chan")
				res.data = append(res.data, d)
				// outputChan will need to be closed if the rc chan was closed
				res.open = open
			case <-done:
				res.done = true
				logger.Debug("dataProcessor: processData", dp, "done, releasing work")
				<-dp.workThrottle
				dp.sendResults()
				exit <- true
				return
			}
		}
	}()
	// do normal data processing, passing in new result chan
	// instead of the original outputChan
	go dp.recordExecution(func() {
		dp.ProcessData(d, rc, killChan)
		done <- true
	})

	// wait on processing to complete
	<-exit
}

// sendResults handles sending work that is completed, as well as
// guaranteeing a FIFO order of the resulting data sent over the
// original outputChan.
func (dp *dataProcessor) sendResults() {
	dp.Lock()
	logger.Debug("dataProcessor: sendResults checking for valid data to send")
	e := dp.workList.Front()
	for e != nil && e.Value.(*result).done {
		logger.Debug("dataHandler: sendResults sending data")
		res := dp.workList.Remove(e).(*result)
		for _, d := range res.data {
			res.outputChan <- d
		}
		if !res.open {
			logger.Debug("dataProcessor: sendResults closing outputChan")
			close(res.outputChan)
		}
		e = dp.workList.Front()
	}
	dp.Unlock()

	if dp.inputClosed && dp.workList.Len() == 0 {
		dp.doneChan <- true
	}
}
