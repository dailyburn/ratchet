package processors

import (
	"bufio"
	"compress/gzip"
	"io"

	"github.com/dailyburn/ratchet/data"
	"github.com/dailyburn/ratchet/util"
)

// IoReader wraps an io.Reader and reads it.
type IoReader struct {
	Reader     io.Reader
	LineByLine bool // defaults to true
	BufferSize int
	Gzipped    bool
}

// NewIoReader returns a new IoReader wrapping the given io.Reader object.
func NewIoReader(reader io.Reader) *IoReader {
	return &IoReader{Reader: reader, LineByLine: true, BufferSize: 1024}
}

func (r *IoReader) ProcessData(d data.Payload, outputChan chan data.Payload, killChan chan error) {
	if r.Gzipped {
		gzReader, err := gzip.NewReader(r.Reader)
		util.KillPipelineIfErr(err, killChan)
		r.Reader = gzReader
	}
	r.ForEachData(killChan, func(d data.Payload) {
		outputChan <- d
	})
}

func (r *IoReader) Finish(outputChan chan data.Payload, killChan chan error) {
}

func (r *IoReader) ForEachData(killChan chan error, foo func(d data.Payload)) {
	if r.LineByLine {
		r.scanLines(killChan, foo)
	} else {
		r.bufferedRead(killChan, foo)
	}
}

func (r *IoReader) scanLines(killChan chan error, forEach func(d data.Payload)) {
	scanner := bufio.NewScanner(r.Reader)
	for scanner.Scan() {
		forEach(data.NewTextPayload(scanner.Text()))
	}
	err := scanner.Err()
	util.KillPipelineIfErr(err, killChan)
}

func (r *IoReader) bufferedRead(killChan chan error, forEach func(d data.Payload)) {
	reader := bufio.NewReader(r.Reader)
	d := make([]byte, r.BufferSize)
	for {
		n, err := reader.Read(d)
		if err != nil && err != io.EOF {
			killChan <- err
		}
		if n == 0 {
			break
		}
		forEach(data.NewBinaryPayload(d))
	}
}

func (r *IoReader) String() string {
	return "IoReader"
}
