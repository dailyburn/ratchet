package processors

import (
	"io/ioutil"

	"github.com/dailyburn/ratchet/data"
	"github.com/dailyburn/ratchet/util"
)

// FileReader opens and reads the contents of the given filename.
type FileReader struct {
	filename string
}

// NewFileReader returns a new FileReader that will read the entire contents
// of the given file path and send it at once. For buffered or line-by-line
// reading try using IoReader.
func NewFileReader(filename string) *FileReader {
	return &FileReader{filename: filename}
}

// ProcessData reads a file and sends its contents to outputChan
func (r *FileReader) ProcessData(d data.JSON, outputChan chan data.JSON, killChan chan error) {
	d, err := ioutil.ReadFile(r.filename)
	util.KillPipelineIfErr(err, killChan)
	outputChan <- d
}

// Finish - see interface for documentation.
func (r *FileReader) Finish(outputChan chan data.JSON, killChan chan error) {
}

func (r *FileReader) String() string {
	return "FileReader"
}
