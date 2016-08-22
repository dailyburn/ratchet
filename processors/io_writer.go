package processors

import (
	"fmt"
	"io"

	"github.com/plandem/ratchet/data"
	"github.com/dailyburn/ratchet/logger"
	"github.com/dailyburn/ratchet/util"
)

// IoWriter wraps any io.Writer object.
// It can be used to write data out to a File, os.Stdout, or
// any other task that can be supported via io.Writer.
type IoWriter struct {
	Writer     io.Writer
	AddNewline bool
}

// NewIoWriter returns a new IoWriter wrapping the given io.Writer object
func NewIoWriter(writer io.Writer) *IoWriter {
	return &IoWriter{Writer: writer, AddNewline: false}
}

func (w *IoWriter) ProcessData(d data.Payload, outputChan chan data.Payload, killChan chan error) {
	var bytesWritten int
	var err error
	if w.AddNewline {
		bytesWritten, err = fmt.Fprintln(w.Writer, string(d))
	} else {
		bytesWritten, err = w.Writer.Write(d)
	}
	util.KillPipelineIfErr(err, killChan)
	logger.Debug("IoWriter:", bytesWritten, "bytes written")
}

func (w *IoWriter) Finish(outputChan chan data.Payload, killChan chan error) {
}

func (w *IoWriter) String() string {
	return "IoWriter"
}
