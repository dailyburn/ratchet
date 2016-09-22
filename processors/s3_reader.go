package processors

// http://docs.aws.amazon.com/sdk-for-go/api/service/s3/S3.html

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/dailyburn/ratchet/data"
	"github.com/dailyburn/ratchet/logger"
	"github.com/dailyburn/ratchet/util"
)

// S3Reader handles retrieving objects from S3. Use NewS3ObjectReader to read
// a single object, or NewS3PrefixReader to read all objects matching the same
// prefix in your bucket.
// S3Reader embeds an IoReeader, so it will support the same configuration
// options as IoReader.
type S3Reader struct {
	IoReader            // embeds IoReader
	bucket              string
	object              string
	prefix              string
	DeleteObjects       bool
	processedObjectKeys []string
	client              *s3.S3
}

// NewS3ObjectReader reads a single object from the given S3 bucket
func NewS3ObjectReader(awsID, awsSecret, awsRegion, bucket, object string) *S3Reader {
	r := S3Reader{bucket: bucket, object: object}
	r.IoReader.LineByLine = true
	creds := credentials.NewStaticCredentials(awsID, awsSecret, "")
	// .WithLogLevel(aws.LogDebugWithRequestRetries | aws.LogDebugWithRequestErrors)
	conf := aws.NewConfig().WithRegion(awsRegion).WithDisableSSL(true).WithCredentials(creds)
	r.client = s3.New(session.New(conf))
	return &r
}

// NewS3PrefixReader reads a all objects from the given S3 bucket that match a prefix.
// See http://docs.aws.amazon.com/AmazonS3/latest/dev/ListingKeysHierarchy.html
// S3 Delimiter will be "/"
func NewS3PrefixReader(awsID, awsSecret, awsRegion, bucket, prefix string) *S3Reader {
	r := NewS3ObjectReader(awsID, awsSecret, awsRegion, bucket, "")
	r.prefix = prefix
	return r
}

// ProcessData reads an entire directory if a prefix is provided (sending each file in that
// directory to outputChan), or just sends the single file to outputChan if a complete
// file path is provided (not a prefix/directory).
//
// It optionally deletes all processed objects once the contents have been sent to outputChan
func (r *S3Reader) ProcessData(d data.JSON, outputChan chan data.JSON, killChan chan error) {
	if r.prefix != "" {
		logger.Debug("S3Reader: process data for prefix", r.prefix)
		objects, err := util.ListS3Objects(r.client, r.bucket, r.prefix)
		logger.Debug("S3Reader: list =", objects)
		util.KillPipelineIfErr(err, killChan)
		for _, o := range objects {
			obj, err := util.GetS3Object(r.client, r.bucket, o)
			util.KillPipelineIfErr(err, killChan)
			r.processObject(obj, outputChan, killChan)
			r.processedObjectKeys = append(r.processedObjectKeys, o)
		}
	} else {
		logger.Debug("S3Reader: process data for object", r.object)
		obj, err := util.GetS3Object(r.client, r.bucket, r.object)
		util.KillPipelineIfErr(err, killChan)
		r.processObject(obj, outputChan, killChan)
		r.processedObjectKeys = append(r.processedObjectKeys, r.object)
	}
	if r.DeleteObjects {
		_, err := util.DeleteS3Objects(r.client, r.bucket, r.processedObjectKeys)
		util.KillPipelineIfErr(err, killChan)
	}
}

// Finish - see interface for documentation.
func (r *S3Reader) Finish(outputChan chan data.JSON, killChan chan error) {
}

func (r *S3Reader) processObject(obj *s3.GetObjectOutput, outputChan chan data.JSON, killChan chan error) {
	// Use IoReader for actual data handling
	r.IoReader.Reader = obj.Body
	r.IoReader.ProcessData(nil, outputChan, killChan)
	obj.Body.Close()
}

func (r *S3Reader) String() string {
	return "S3Reader"
}
