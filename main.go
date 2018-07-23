package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/silverstripeltd/ssp-sdk-go/ssp"
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
	fmt.Printf("tape %s\n", version)

	if len(os.Args) != 4 {
		fatalErr(fmt.Errorf("usage: %s path/to/src/directory s3://bucket/destination/file.tar.gz http://platform/ \n", os.Args[0]))
	}

	// wrap all complicated argument validation and parsing in a config
	// @todo, return error
	conf := newConfig(os.Args)

	files, err := scanDirectory(conf.src)
	if err != nil {
		fatalErr(err)
	}

	// use a pipe between the tar compressing and S3 uploading so that we don't have to waste diskspace
	reader, writer := io.Pipe()

	// spin off into a go routine so that the upload can stream the output from this into the uploader via the writer
	go func() {
		err := buildPackage(conf.tarPrefix, writer, files)
		if err != nil {
			fatalErr(err)
		}
	}()

	preSignedURL, err := upload(reader, conf)
	if err != nil {
		fatalErr(err)
	}

	if err := createDeployment(conf, preSignedURL); err != nil {
		fatalErr(err)
	}
}

func upload(source io.ReadCloser, conf config) (string, error) {
	defer source.Close()

	fmt.Println("[-] streaming compressed tar archive to s3")

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(conf.s3Region),
	}))

	svc := s3.New(sess)
	// Create an uploader (can do multipart) with S3 client and default options
	uploader := s3manager.NewUploaderWithClient(svc)
	params := &s3manager.UploadInput{
		Bucket:      aws.String(conf.s3Bucket),
		Key:         aws.String(conf.s3Key),
		ContentType: aws.String("application/x-tgz"),
		Body:        source,
	}

	if _, err := uploader.Upload(params); err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "AccessDenied":
				return "", fmt.Errorf("uploading failed: Access denied")
			default:
				return "", fmt.Errorf("uploading failed: %s", aerr.Message())
			}
		}
		return "", err
	}
	fmt.Println("[-] streaming complete")

	fmt.Println("[-] requesting pre-signed link to the S3 object")
	req, _ := svc.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(conf.s3Bucket),
		Key:    aws.String(conf.s3Key),
	})

	preSignedURL, err := req.Presign(300 * time.Second)
	if err != nil {
		return "", err
	}

	return preSignedURL, nil
}

func fatalErr(err error) {
	fmt.Fprintf(os.Stderr, "[!] %s\n", err.Error())
	os.Exit(1)
}

func createDeployment(conf config, packageURL string) error {

	fmt.Printf("[-] requesting deployment from %s\n", conf.dash.Host)
	client, err := ssp.NewClient(&ssp.Config{
		Email:   os.Getenv("DASHBOARD_USER"),
		Token:   os.Getenv("DASHBOARD_TOKEN"),
		BaseURL: fmt.Sprintf("%s://%s", conf.dash.Scheme, conf.dash.Host),
	})
	if err != nil {
		return err
	}

	dep, err := client.CreateDeployment(conf.stack, conf.env, &ssp.CreateDeployment{
		Ref:     packageURL,
		RefType: "package",
		Title:   "[CI] Deploy",
		Summary: "",
		Options: []string{""},
		Bypass:  false,
	})

	if err != nil {
		return err
	}

	fmt.Println(dep)

	return nil
}
