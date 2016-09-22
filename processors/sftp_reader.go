package processors

import (
	"github.com/dailyburn/ratchet/data"
	"github.com/dailyburn/ratchet/util"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SftpReader reads a single object at a given path, or walks through the
// directory specified by the path (SftpReader.Walk must be set to true).
//
// To only send full paths (and not file contents), set FileNamesOnly to true.
// If FileNamesOnly is set to true, DeleteObjects will be ignored.
type SftpReader struct {
	IoReader      // embeds IoReader
	parameters    *util.SftpParameters
	client        *sftp.Client
	DeleteObjects bool
	Walk          bool
	FileNamesOnly bool
	initialized   bool
	CloseOnFinish bool
}

// NewSftpReader instantiates a new sftp reader, a connection to the remote server is delayed until data is recv'd by the reader
// By default, the connection to the remote client will be closed in the Finish() func.
// Set CloseOnFinish to false to manage the connection manually.
func NewSftpReader(server string, username string, path string, authMethods ...ssh.AuthMethod) *SftpReader {
	r := SftpReader{
		parameters: &util.SftpParameters{
			Server:      server,
			Username:    username,
			Path:        path,
			AuthMethods: authMethods,
		},
		initialized:   false,
		DeleteObjects: false,
		FileNamesOnly: false,
		CloseOnFinish: true,
	}
	r.IoReader.LineByLine = true
	return &r
}

// NewSftpReaderByClient instantiates a new sftp reader using an existing connection to the remote server.
// By default, the connection to the remote client will *not* be closed in the Finish() func.
// Set CloseOnFinish to true to have this processor clean up the connection when it's done.
func NewSftpReaderByClient(client *sftp.Client, path string) *SftpReader {
	r := SftpReader{
		parameters:    &util.SftpParameters{Path: path},
		client:        client,
		initialized:   true,
		DeleteObjects: false,
		FileNamesOnly: false,
		CloseOnFinish: false,
	}
	r.IoReader.LineByLine = true
	return &r
}

// ProcessData optionally walks through the tree to send each object separately, or sends the single
// object upstream
func (r *SftpReader) ProcessData(d data.JSON, outputChan chan data.JSON, killChan chan error) {
	r.ensureInitialized(killChan)
	if r.Walk {
		r.walk(outputChan, killChan)
	} else {
		r.sendObject(r.parameters.Path, outputChan, killChan)
	}
}

// Finish optionally closes open references to the remote server
func (r *SftpReader) Finish(outputChan chan data.JSON, killChan chan error) {
	if r.CloseOnFinish {
		r.CloseClient()
	}
}

// CloseClient allows you to manually close the connection to the remote client (as the remote client
// itself is not exported)
func (r *SftpReader) CloseClient() {
	r.client.Close()
}

func (r *SftpReader) String() string {
	return "SftpReader"
}

func (r *SftpReader) ensureInitialized(killChan chan error) {
	if r.initialized {
		return
	}

	client, err := util.SftpClient(r.parameters.Server, r.parameters.Username, r.parameters.AuthMethods)
	util.KillPipelineIfErr(err, killChan)

	r.client = client
	r.initialized = true
}

func (r *SftpReader) walk(outputChan chan data.JSON, killChan chan error) {
	walker := r.client.Walk(r.parameters.Path)
	for walker.Step() {
		util.KillPipelineIfErr(walker.Err(), killChan)
		if !walker.Stat().IsDir() {
			r.sendObject(walker.Path(), outputChan, killChan)
		}
	}
}

func (r *SftpReader) sendObject(path string, outputChan chan data.JSON, killChan chan error) {
	if r.FileNamesOnly {
		r.sendFilePath(path, outputChan, killChan)
	} else {
		r.sendFile(path, outputChan, killChan)
	}
}

func (r *SftpReader) sendFilePath(path string, outputChan chan data.JSON, killChan chan error) {
	sftpPath := util.SftpPath{Path: path}
	d, err := data.NewJSON(sftpPath)
	util.KillPipelineIfErr(err, killChan)
	outputChan <- d
}

func (r *SftpReader) sendFile(path string, outputChan chan data.JSON, killChan chan error) {
	file, err := r.client.Open(path)

	util.KillPipelineIfErr(err, killChan)
	defer file.Close()

	r.IoReader.Reader = file
	r.IoReader.ProcessData(nil, outputChan, killChan)

	if r.DeleteObjects {
		err = r.client.Remove(path)
		util.KillPipelineIfErr(err, killChan)
	}
}
