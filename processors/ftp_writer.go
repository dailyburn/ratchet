package processors

import (
	"io"

	"github.com/dailyburn/ratchet/data"
	"github.com/dailyburn/ratchet/logger"
	"github.com/dailyburn/ratchet/util"
	"github.com/jlaffaye/ftp"
)

// FtpWriter type represents an ftp writter processor
type FtpWriter struct {
	ftpFilepath   string
	conn          *ftp.ServerConn
	fileWriter    *io.PipeWriter
	authenticated bool
	host          string
	username      string
	password      string
	path          string
}

// NewFtpWriter instantiates new instance of an ftp writer
func NewFtpWriter(host, username, password, path string) *FtpWriter {
	return &FtpWriter{authenticated: false, host: host, username: username, password: password, path: path}
}

// connect - opens a connection to the provided ftp host and then authenticates with the host with the username, password attributes
func (f *FtpWriter) connect(killChan chan error) {
	conn, err := ftp.Dial(f.host)
	if err != nil {
		util.KillPipelineIfErr(err, killChan)
	}

	lerr := conn.Login(f.username, f.password)
	if lerr != nil {
		util.KillPipelineIfErr(lerr, killChan)
	}

	r, w := io.Pipe()

	f.conn = conn
	go f.conn.Stor(f.path, r)
	f.fileWriter = w
	f.authenticated = true
}

// ProcessData writes data as is directly to the output file
func (f *FtpWriter) ProcessData(d data.JSON, outputChan chan data.JSON, killChan chan error) {
	logger.Debug("FTPWriter Process data:", string(d))
	if !f.authenticated {
		f.connect(killChan)
	}

	_, e := f.fileWriter.Write([]byte(d))
	if e != nil {
		util.KillPipelineIfErr(e, killChan)
	}
}

// Finish closes open references to the remote file and server
func (f *FtpWriter) Finish(outputChan chan data.JSON, killChan chan error) {
	if f.fileWriter != nil {
		f.fileWriter.Close()
	}
	if f.conn != nil {
		f.conn.Logout()
		f.conn.Quit()
	}
}

func (f *FtpWriter) String() string {
	return "FtpWriter"
}
