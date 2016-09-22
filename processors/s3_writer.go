package processors

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/dailyburn/ratchet/data"
	"github.com/dailyburn/ratchet/util"
)

// S3Writer sends data upstream to S3. By default, we will not compress data before sending it.
// Set the `Compress` flag to true to use gzip compression before storing in S3 (if this flag is
// set to true, ".gz" will automatically be appended to the key name specified).
//
// By default, we will separate each iteration of data sent to `ProcessData` with a new line
// when we piece back together to send to S3. Change the `LineSeparator` attribute to change
// this behavior.
type S3Writer struct {
	data          []string
	Compress      bool
	LineSeparator string
	config        *aws.Config
	bucket        string
	key           string
}

// NewS3Writer instaniates a new S3Writer
func NewS3Writer(awsID, awsSecret, awsRegion, bucket, key string) *S3Writer {
	w := S3Writer{bucket: bucket, key: key, LineSeparator: "\n", Compress: false}

	creds := credentials.NewStaticCredentials(awsID, awsSecret, "")
	// .WithLogLevel(aws.LogDebugWithRequestRetries | aws.LogDebugWithRequestErrors)
	w.config = aws.NewConfig().WithRegion(awsRegion).WithDisableSSL(true).WithCredentials(creds)

	return &w
}

// ProcessData enqueues all received data
func (w *S3Writer) ProcessData(d data.JSON, outputChan chan data.JSON, killChan chan error) {
	w.data = append(w.data, string(d))
}

// Finish writes all enqueued data to S3, defering to util.WriteS3Object
func (w *S3Writer) Finish(outputChan chan data.JSON, killChan chan error) {
	util.WriteS3Object(w.data, w.config, w.bucket, w.key, w.LineSeparator, w.Compress)
}

func (w *S3Writer) String() string {
	return "S3Writer"
}
