package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/silverstripeltd/ssp-sdk-go/ssp"
)

// @todo output a link to the deployment after it's been created
// @todo output how long this deployment might take (fast|full)
// @todo allow deploy = true|false argument
// @todo allow Subject line

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
	err := realMain()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] %s\n", err.Error())
		os.Exit(1)
	}
}

func realMain() error {
	ts := time.Now()
	fmt.Printf("tape %s\n", version)

	if len(os.Args) != 4 {
		return fmt.Errorf("usage: %s path/to/src/directory s3://bucket/destination/file.tar.gz https://platform.silverstripe.com/naut/project/MYPROJECT/environment/MYENV \n", os.Args[0])
	}

	// wrap all complicated argument validation and parsing in a config
	// @todo, return error
	conf := newConfig(os.Args)

	files, err := scanDirectory(conf.src)
	if err != nil {
		return err
	}

	// use a pipe between the tar compressing and S3 uploading so that we don't have to waste diskspace
	reader, writer := io.Pipe()

	// spin off into a go routine so that the upload can stream the output from this into the uploader via the writer
	go func() {
		err := buildPackage(conf.tarPrefix, writer, files)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[!] %s\n", err.Error())
		}
	}()

	preSignedURL, err := upload(reader, conf)
	if err != nil {
		return err
	}

	dep, err := createDeployment(conf, preSignedURL)
	if err != nil {
		return err
	}

	dep, err = startDeployment(conf, dep)
	if err != nil {
		return err
	}

	if _, err := waitForDeployResult(conf, dep); err != nil {
		return err
	}

	fmt.Println("\n[=] deployment successful! 🍺")
	fmt.Printf("    %s\n", time.Since(ts))
	return nil
}

func upload(source io.ReadCloser, conf config) (string, error) {
	defer source.Close()

	svc := conf.S3Client()

	// Create an uploader (can do multipart) with S3 client and default options
	uploader := s3manager.NewUploaderWithClient(svc)
	params := &s3manager.UploadInput{
		Bucket:      aws.String(conf.s3.bucket),
		Key:         aws.String(conf.s3.key),
		ContentType: aws.String("application/x-tgz"),
		Body:        source,
	}

	_, err := uploader.Upload(params)

	// try to parse AWS errors so they are not so but ugly to display
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "AccessDenied":
				return "", fmt.Errorf("uploading failed: Access denied for %s", conf.s3.bucket)
			default:
				return "", fmt.Errorf("uploading failed: %s", aerr.Message())
			}
		}
		return "", err
	}

	fmt.Println("[-] S3 upload completed")
	fmt.Println("[-] requesting pre-signed link to the S3 object")

	req, _ := svc.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(conf.s3.bucket),
		Key:    aws.String(conf.s3.key),
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

	fmt.Printf("[-] requesting deployment from %s\n", conf.dashboard.url.Host)
	client, err := conf.dashboardClient()
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

	client, err := conf.dashboardClient()
	if err != nil {
		return nil, err
	}
	req := &ssp.StartDeployment{ID: d.ID}
	d, err = client.StartDeployment(d.Stack.ID, d.Environment.ID, req)
	return d, err
}

func waitForDeployResult(conf config, d *ssp.Deployment) (*ssp.Deployment, error) {
	client, err := conf.dashboardClient()
	if err != nil {
		return nil, err
	}

	var state ssp.State = ssp.StateNew

	progressTick := time.NewTicker(time.Minute)
	checkTick := time.NewTicker(time.Second * 5)
	cancelTick := time.NewTicker(time.Minute * 25)

	for {
		select {
		// need to output something so that CodeShip doesn't cancel the build due to no output
		case <-progressTick.C:
			fmt.Printf("[-] deployment currently in state '%s'\n", d.State)

		case <-checkTick.C:
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
				return d, fmt.Errorf("deployment failed, check logs at %s\n", conf.dashboard.url.String())
			}
			if d.State == ssp.StateCompleted {
				return d, nil
			}

		case <-cancelTick.C:
			return d, fmt.Errorf("waiting for deployment to finish timed out, check logs at %s\n", conf.dashboard.url.String())
		}
	}

	return d, nil
}
