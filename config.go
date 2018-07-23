package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type config struct {
	src           string
	tarPrefix     string
	packagePrefix string
	stack, env    string
	dash          *url.URL

	s3Bucket string
	s3Key    string
	s3Region string
}

func newConfig(args []string) config {
	c := config{}

	var err error

	if c.src, err = filepath.Abs(args[1]); err != nil {
		fatalErr(err)
	}

	c.tarPrefix = filepath.Base(c.src)

	if c.dash, err = url.Parse(args[3]); err != nil {
		fatalErr(err)
	}

	// grab the stack and env from the dashboard url
	pathParts := strings.Split(strings.Trim(c.dash.Path, "/"), "/")
	if len(pathParts) != 5 {
		fatalErr(fmt.Errorf("usage: %s path/to/src/directory s3://bucket/destination/file.tar.gz http://platform/ \n", os.Args[0]))
	}
	c.stack, c.env = pathParts[2], pathParts[4]

	s3URL, err := url.Parse(args[2])
	if err != nil {
		fatalErr(err)
	}

	if s3URL.Scheme != "s3" {
		fatalErr(fmt.Errorf("S3Uri argument does not have valid protocol, should be 's3'"))
	}
	if s3URL.Host == "" {
		fatalErr(fmt.Errorf("S3Uri is missing bucket name"))
	}

	c.s3Bucket = s3URL.Host
	c.s3Key = s3URL.Path

	sess := session.Must(session.NewSession(&aws.Config{Region: aws.String("ap-southeast-2")}))
	region, err := s3manager.GetBucketRegion(context.Background(), sess, c.s3Bucket, "ap-southeast-2")
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "NotFound" {
			fatalErr(fmt.Errorf("unable to find bucket %s's region", c.s3Bucket))
		}
		fatalErr(err)
	}
	c.s3Region = region

	return c
}
