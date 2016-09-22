package util

import (
	"compress/gzip"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/dailyburn/ratchet/logger"
)

// ListS3Objects returns all object keys matching the given prefix. Note that
// delimiter is set to "/". See http://docs.aws.amazon.com/AmazonS3/latest/dev/ListingKeysHierarchy.html
func ListS3Objects(client *s3.S3, bucket, keyPrefix string) ([]string, error) {
	logger.Debug("ListS3Objects: ", bucket, "-", keyPrefix)
	params := &s3.ListObjectsInput{
		Bucket:    aws.String(bucket), // Required
		Delimiter: aws.String("/"),
		// EncodingType: aws.String("EncodingType"),
		// Marker:       aws.String("Marker"),
		MaxKeys: aws.Int64(1000),
		Prefix:  aws.String(keyPrefix),
	}

	objects := []string{}
	err := client.ListObjectsPages(params, func(page *s3.ListObjectsOutput, lastPage bool) bool {
		for _, o := range page.Contents {
			objects = append(objects, *o.Key)
		}
		return lastPage
	})
	if err != nil {
		return nil, err
	}

	return objects, nil
}

// GetS3Object returns the object output for the given object key
func GetS3Object(client *s3.S3, bucket, objKey string) (*s3.GetObjectOutput, error) {
	logger.Debug("GetS3Object: ", bucket, "-", objKey)
	params := &s3.GetObjectInput{
		Bucket: aws.String(bucket), // Required
		Key:    aws.String(objKey), // Required
		// IfMatch:                    aws.String("IfMatch"),
		// IfModifiedSince:            aws.Time(time.Now()),
		// IfNoneMatch:                aws.String("IfNoneMatch"),
		// IfUnmodifiedSince:          aws.Time(time.Now()),
		// Range:                      aws.String("Range"),
		// RequestPayer:               aws.String("RequestPayer"),
		// ResponseCacheControl:       aws.String("ResponseCacheControl"),
		// ResponseContentDisposition: aws.String("ResponseContentDisposition"),
		// ResponseContentEncoding:    aws.String("ResponseContentEncoding"),
		// ResponseContentLanguage:    aws.String("ResponseContentLanguage"),
		// ResponseContentType:        aws.String("ResponseContentType"),
		// ResponseExpires:            aws.Time(time.Now()),
		// SSECustomerAlgorithm:       aws.String("SSECustomerAlgorithm"),
		// SSECustomerKey:             aws.String("SSECustomerKey"),
		// SSECustomerKeyMD5:          aws.String("SSECustomerKeyMD5"),
		// VersionId:                  aws.String("ObjectVersionId"),
	}

	return client.GetObject(params)
}

// DeleteS3Objects deletes the objects specified by the given object keys
func DeleteS3Objects(client *s3.S3, bucket string, objKeys []string) (*s3.DeleteObjectsOutput, error) {
	logger.Debug("DeleteS3Objects: ", bucket, "-", objKeys)
	s3Ids := make([]*s3.ObjectIdentifier, len(objKeys))
	for i, key := range objKeys {
		s3Ids[i] = &s3.ObjectIdentifier{Key: aws.String(key)}
	}

	params := &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket), // Required
		Delete: &s3.Delete{ // Required
			Objects: s3Ids,
			Quiet:   aws.Bool(true),
		},
		// MFA:          aws.String("MFA"),
		// RequestPayer: aws.String("RequestPayer"),
	}

	return client.DeleteObjects(params)
}

// WriteS3Object writes the data to the given key, optionally compressing it first
func WriteS3Object(data []string, config *aws.Config, bucket string, key string, lineSeparator string, compress bool) (string, error) {
	var reader io.Reader

	byteReader := strings.NewReader(strings.Join(data, lineSeparator))

	if compress {
		key = fmt.Sprintf("%v.gz", key)
		pipeReader, pipeWriter := io.Pipe()
		reader = pipeReader

		go func() {
			gw := gzip.NewWriter(pipeWriter)
			io.Copy(gw, byteReader)
			gw.Close()
			pipeWriter.Close()
		}()
	} else {
		reader = byteReader
	}

	uploader := s3manager.NewUploader(session.New(config))

	result, err := uploader.Upload(&s3manager.UploadInput{
		Body:   reader,
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	return result.Location, err
}
