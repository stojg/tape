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
	"time"
	"github.com/silverstripeltd/ssp-sdk-go/ssp"
	"strings"
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

	if len(os.Args) != 4 {
		fatalErr(fmt.Errorf("usage: %s path/to/src/directory s3://bucket/destination/file.tar.gz http://platform/ \n", os.Args[0]))
	}

	src, err := filepath.Abs(os.Args[1])
	if err != nil {
		fatalErr(err)
	}

	dashboardURL, err := url.Parse(os.Args[3])
	if err != nil {
		fatalErr(err)
	}

	dParts := strings.Split(strings.Trim(dashboardURL.Path, "/"), "/")
	if len(dParts) != 5 {
		fmt.Println(len(dParts), dParts);
		fatalErr(fmt.Errorf("usage: %s path/to/src/directory s3://bucket/destination/file.tar.gz http://platform/ \n", os.Args[0]))
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

	preSignedURL, err := upload(reader, s3URL, region)
	if err != nil {
		fatalErr(err)
	}

	if err := createDeployment(dashboardURL.Scheme, dashboardURL.Host, dParts[2], dParts[4], preSignedURL); err != nil {
		fatalErr(err)
	}
}

func createDeployment(scheme, host, stack, env, packageURL string) error {
	client, err := ssp.NewClient(&ssp.Config{
		Email:   os.Getenv("DASHBOARD_USER"),
		Token:   os.Getenv("DASHBOARD_TOKEN"),
		BaseURL: fmt.Sprintf("%s://%s", scheme, host),
	})
	if err != nil {
		return err
	}

	dep, err := client.CreateDeployment(stack, env, &ssp.CreateDeployment{
		Ref:     packageURL,
		RefType: "package",
		Title:   "My test",
		Summary: "very deploy, much nice",
		Options: []string{""},
		Bypass:  false,
	})

	if err != nil {
		return err
	}

	fmt.Println(dep)

	return nil
}

func fatalErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func upload(source io.Reader, dest *url.URL, awsRegion string) (string, error) {

	contentType := "application/x-tgz"

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	}))

	svc := s3.New(sess)
	// Create an uploader (can do multipart) with S3 client and default options
	uploader := s3manager.NewUploaderWithClient(svc)
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
				return "", fmt.Errorf("uploading to %s: Access denied", dest)
			default:
				return "", fmt.Errorf("uploading to %s: %s", dest, aerr.Message())
			}
		}
		return "", err
	}

	fmt.Printf("uploaded %s\n", dest)

	req, _ := svc.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(dest.Host),
		Key:    aws.String(dest.Path),
	})
	presignedURL, err := req.Presign(300 * time.Second)
	if err != nil {
		return "", err
	}

	return presignedURL, nil
}
