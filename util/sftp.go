package util

import (
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SftpParameters is used for storing connection parameters for later executing sftp commands
type SftpParameters struct {
	Server      string
	Username    string
	Path        string
	AuthMethods []ssh.AuthMethod
}

// SftpPath is a simple struct for storing the full path of an object
type SftpPath struct {
	Path string `json:"path,omitempty"`
}

// FileName defers to filepath.Base
func (t SftpPath) FileName() string {
	return filepath.Base(t.Path)
}

// SftpClient sets up and return the client
func SftpClient(server string, username string, authMethod []ssh.AuthMethod, opts ...sftp.ClientOption) (*sftp.Client, error) {
	var client *sftp.Client

	config := &ssh.ClientConfig{
		User: username,
		Auth: authMethod,
	}

	conn, err := ssh.Dial("tcp", server, config)
	if err != nil {
		return client, err
	}

	return sftp.NewClient(conn, opts...)
}

// SftpKeyAuth generates an ssh.AuthMethod given the path of a private key
func SftpKeyAuth(privateKeyPath string) (auth ssh.AuthMethod, err error) {
	privateKey, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return
	}

	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return
	}

	auth = ssh.PublicKeys(signer)

	return
}
