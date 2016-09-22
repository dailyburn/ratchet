package processors

import (
	"io"

	"github.com/dailyburn/ratchet/data"
	"github.com/dailyburn/ratchet/util"
)

// CSVWriter is handles converting data.JSON objects into CSV format,
// and writing them to the given io.Writer. The Data
// must be a valid JSON object or a slice of valid JSON objects.
// If you already have Data formatted as a CSV string you can
// use an IoWriter instead.
type CSVWriter struct {
	Parameters util.CSVParameters
}

// NewCSVWriter returns a new CSVWriter wrapping the given io.Writer object
func NewCSVWriter(w io.Writer) *CSVWriter {
	writer := util.NewCSVWriter()
	writer.SetWriter(w)

	return &CSVWriter{
		Parameters: util.CSVParameters{
			Writer:        writer,
			WriteHeader:   true,
			HeaderWritten: false,
			SendUpstream:  false,
		},
	}
}

// ProcessData defers to util.CSVProcess
func (w *CSVWriter) ProcessData(d data.JSON, outputChan chan data.JSON, killChan chan error) {
	util.CSVProcess(&w.Parameters, d, outputChan, killChan)
}

// Finish - see interface for documentation.
func (w *CSVWriter) Finish(outputChan chan data.JSON, killChan chan error) {
}

func (w *CSVWriter) String() string {
	return "CSVWriter"
}
