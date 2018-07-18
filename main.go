package main

import (
	"fmt"
	"os"
	"path/filepath"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/aws/session"
	"net/url"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"context"
	"io"
)

var (
	version string
)

type FileStat struct {
	src  string
	path string
	link string
	stat os.FileInfo
	err  error
}

func main() {
	fmt.Printf("Tape %s\n", version)

	if len(os.Args) != 3 {
		fatalErr(fmt.Errorf("usage: %s path/to/src/directory s3://bucket/destination/file.tar.gz\n", os.Args[0]))
	}

	src, err := filepath.Abs(os.Args[1])
	if err != nil {
		fatalErr(err)
	}

	packagePrefix := filepath.Base(src)

	srcFiles, err := scanDirectory(src)
	if err != nil {
		fatalErr(err)
	}

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String("ap-southeast-1"),
	}))

	s3URL, err := url.Parse(os.Args[2])
	if err != nil {
		fatalErr(err)
	}
	if s3URL.Scheme != "s3" {
		fatalErr(fmt.Errorf("S3Uri argument does not have valid protocol, should be 's3'"))
	}
	if s3URL.Host == "" {
		fatalErr(fmt.Errorf("S3Uri is missing bucket name"))
	}

	fmt.Printf("Uploading %s as a compressed tar to %s\n", src, s3URL)

	region, err := s3manager.GetBucketRegion(context.Background(), sess, s3URL.Host, "ap-southeast-2")
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "NotFound" {
			fatalErr(fmt.Errorf("unable to find bucket %s's region", s3URL.Host))
		}
		fatalErr(err)
	}

	reader, writer := io.Pipe()
	defer reader.Close()

	go func() {
		defer writer.Close()
		err = buildPackage(packagePrefix, writer, srcFiles)
		if err != nil {
			fatalErr(err)
		}
	}()

	err = upload(reader, s3URL, region)
	if err != nil {
		fatalErr(err)
	}
}

func fatalErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func upload(source io.Reader, dest *url.URL, awsRegion string) error {

	contentType := "application/x-tgz"

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	}))

	srv  := s3.New(sess)
	// Create an uploader (can do multipart) with S3 client and default options
	uploader := s3manager.NewUploaderWithClient(srv)
	params := &s3manager.UploadInput{
		Bucket:      aws.String(dest.Host),
		Key:         aws.String(dest.Path),
		Body:        source,
		ContentType: aws.String(contentType),
	}

	if _, err := uploader.Upload(params); err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "AccessDenied":
				return fmt.Errorf("uploading to %s: Access denied", dest)
			default:
				return fmt.Errorf("uploading to %s: %s", dest, aerr.Message())
			}
		}
		return err
	}

	fmt.Printf("uploaded %s\n", dest)
	return nil
}
