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
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/silverstripeltd/ssp-sdk-go/ssp"
)

func newConfig(args []string) config {
	c := config{}

	var err error

	if c.src, err = filepath.Abs(args[0]); err != nil {
		handleError(err)
	}

	c.tarPrefix = filepath.Base(c.src)

	if c.dashboard.url, err = url.Parse(args[2]); err != nil {
		handleError(err)
	}

	// grab the stack and env from the dashboard url
	pathParts := strings.Split(strings.Trim(c.dashboard.url.Path, "/"), "/")
	if len(pathParts) != 5 {
		handleError(fmt.Errorf("usage: %s path/to/src/directory s3://bucket/destination/file.tar.gz http://platform/ \n", os.Args[0]))
	}
	c.stack, c.env = pathParts[2], pathParts[4]

	c.dashboard.user = os.Getenv("DASHBOARD_USER")
	c.dashboard.token = os.Getenv("DASHBOARD_TOKEN")

	s3URL, err := url.Parse(args[1])
	if err != nil {
		handleError(err)
	}

	if s3URL.Scheme != "s3" {
		handleError(fmt.Errorf("S3Uri argument does not have valid protocol, should be 's3'"))
	}
	if s3URL.Host == "" {
		handleError(fmt.Errorf("S3Uri is missing bucket name"))
	}

	c.s3.bucket = s3URL.Host
	c.s3.key = s3URL.Path
	c.s3.region = bucketRegion(c.s3.bucket)

	return c
}

func bucketRegion(bucket string) string {
	sess := session.Must(session.NewSession(&aws.Config{Region: aws.String("ap-southeast-2")}))
	region, err := s3manager.GetBucketRegion(context.Background(), sess, bucket, "ap-southeast-2")
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "NotFound" {
			handleError(fmt.Errorf("unable to find bucket %s's region", bucket))
		}
		handleError(err)
	}
	return region
}

type config struct {
	src       string
	tarPrefix string

	stack, env string
	dashboard  struct {
		url   *url.URL
		user  string
		token string
	}

	s3 struct {
		bucket string
		key    string
		region string
	}
}

func (conf config) dashboardClient() (*ssp.Client, error) {
	return ssp.NewClient(&ssp.Config{
		Email:   conf.dashboard.user,
		Token:   conf.dashboard.token,
		BaseURL: fmt.Sprintf("%s://%s", conf.dashboard.url.Scheme, conf.dashboard.url.Host),
	})
}

func (conf config) S3Client() *s3.S3 {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(conf.s3.region),
	}))
	svc := s3.New(sess)
	return svc
}
