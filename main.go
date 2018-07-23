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

	dep, err := createDeployment(conf, preSignedURL)
	if err != nil {
		fatalErr(err)
	}

	dep, err = startDeployment(conf, dep)
	if err != nil {
		fatalErr(err)
	}

	if _, err := waitForDeployResult(conf, dep); err != nil {
		fatalErr(err)
	}

	fmt.Println("\n[=] deployment successful! 🍺")
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

func createDeployment(conf config, packageURL string) (*ssp.Deployment, error) {

	fmt.Printf("[-] requesting deployment from %s\n", conf.dash.Host)
	client, err := ssp.NewClient(&ssp.Config{
		Email:   os.Getenv("DASHBOARD_USER"),
		Token:   os.Getenv("DASHBOARD_TOKEN"),
		BaseURL: fmt.Sprintf("%s://%s", conf.dash.Scheme, conf.dash.Host),
	})
	if err != nil {
		return nil, err
	}

	dep, err := client.CreateDeployment(conf.stack, conf.env, &ssp.CreateDeployment{
		Ref:     packageURL,
		RefType: "package",
		Title:   "[CI] Deploy",
		Summary: "",
		Options: []string{""},
		Bypass:  true,
	})

	if err != nil {
		return dep, err
	}

	return dep, nil
}

func startDeployment(conf config, d *ssp.Deployment) (*ssp.Deployment, error) {
	fmt.Printf("[-] starting deployment %d\n", d.ID)

	client, err := ssp.NewClient(&ssp.Config{
		Email:   os.Getenv("DASHBOARD_USER"),
		Token:   os.Getenv("DASHBOARD_TOKEN"),
		BaseURL: fmt.Sprintf("%s://%s", conf.dash.Scheme, conf.dash.Host),
	})
	if err != nil {
		return d, err
	}

	req := &ssp.StartDeployment{ID: d.ID}
	d, err = client.StartDeployment(d.Stack.ID, d.Environment.ID, req)
	return d, err
}

func waitForDeployResult(conf config, d *ssp.Deployment) (*ssp.Deployment, error) {

	fmt.Printf("[-] waiting for result of deployment %d\n", d.ID)

	client, err := ssp.NewClient(&ssp.Config{
		Email:   os.Getenv("DASHBOARD_USER"),
		Token:   os.Getenv("DASHBOARD_TOKEN"),
		BaseURL: fmt.Sprintf("%s://%s", conf.dash.Scheme, conf.dash.Host),
	})
	if err != nil {
		return d, err
	}

	var waited time.Duration
	ts := time.Now()
	var state ssp.State = ssp.StateNew

	tick := time.NewTicker(time.Second * 5)
	for range tick.C {
		d, err = client.GetDeployment(d.Stack.ID, d.Environment.ID, fmt.Sprintf("%d", d.ID))
		if err != nil {
			return d, err
		}

		if d.State != state {
			// only display state changes
			fmt.Printf("[-] deployment currently in state '%s'\n", d.State)
			state = d.State
		}

		if d.State == ssp.StateFailed {
			return d, fmt.Errorf("[!] deployment failed, check logs at %s\n", conf.dash.String())
		}
		if d.State == ssp.StateCompleted {
			return d, nil
		}

		waited += time.Since(ts)
		ts = time.Now()

		if waited > time.Minute*20 {
			return d, fmt.Errorf("[!] waiting for deployment to finish timed out, check logs at %s\n", conf.dash.String())
		}
	}

	return d, nil

}
