package main

import (
	"flag"
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

	title := flag.String("title", "[CI] Deploy", "A title to use in the deployments title")
	flag.Parse()

	if len(flag.Args()) != 3 {
		usage()
		os.Exit(1)
	}

	handleError(realMain(*title))
}

func realMain(title string) error {
	startTime := time.Now()
	fmt.Printf("tape %s\n", version)

	// wrap all complicated argument validation and parsing in a config
	// @todo, return error
	conf := newConfig(flag.Args())

	files, err := scanDirectory(conf.src)
	if err != nil {
		return err
	}

	// use a pipe between the tar compressing and S3 uploading so that we don't have to waste disk space
	reader, writer := io.Pipe()

	// spin off into a go routine so that the upload can stream the output from this into the uploader via the writer
	go func() {
		err := buildPackage(conf.tarPrefix, writer, files)
		handleError(err)
	}()

	if err := upload(reader, conf); err != nil {
		return err
	}

	preSignedURL, err := preSignedLink(conf)
	if err != nil {
		return err
	}

	dep, err := createDeployment(conf, preSignedURL, title)
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
	fmt.Printf("    %s\n", time.Since(startTime))
	return nil
}

func upload(source io.ReadCloser, conf config) error {
	// Create an uploader (can do multipart) with S3 client and default options
	uploader := s3manager.NewUploaderWithClient(conf.S3Client())
	params := &s3manager.UploadInput{
		Bucket:      aws.String(conf.s3.bucket),
		Key:         aws.String(conf.s3.key),
		ContentType: aws.String("application/x-tgz"),
		Body:        source,
	}

	fmt.Printf("[-] starting upload to s3://%s/%s\n", conf.s3.bucket, conf.s3.key)
	_, err := uploader.Upload(params)

	handleError(source.Close())

	// try to parse AWS errors so they are not so but ugly to display
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			switch awsErr.Code() {
			case "AccessDenied":
				return fmt.Errorf("uploading to S3 bucket '%s' got an access denied error", conf.s3.bucket)
			default:
				return fmt.Errorf("uploading failed: '%s'", awsErr.Message())
			}
		}
		return err
	}

	fmt.Println("[-] S3 upload completed")
	return nil
}

func preSignedLink(conf config) (string, error) {
	fmt.Println("[-] requesting a 5 minute pre-signed link to the S3 object")
	req, _ := conf.S3Client().GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(conf.s3.bucket),
		Key:    aws.String(conf.s3.key),
	})
	preSignedURL, err := req.Presign(300 * time.Second)
	if err != nil {
		return "", err
	}
	return preSignedURL, nil
}

func handleError(err error) {
	if err == nil {
		return
	}
	if _, err2 := fmt.Fprintf(os.Stderr, "[!] %s\n", err.Error()); err2 != nil {
		panic(err2)
	}
	os.Exit(1)
}

func createDeployment(conf config, packageURL, title string) (*ssp.Deployment, error) {
	fmt.Printf("[-] requesting deployment from %s\n", conf.dashboard.url.Host)
	client, err := conf.dashboardClient()
	if err != nil {
		return nil, err
	}

	dep, err := client.CreateDeployment(conf.stack, conf.env, &ssp.CreateDeployment{
		Ref:     packageURL,
		RefType: "package",
		Title:   title,
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
}

func usage() {
	format := `"usage: %s [--title \"my deploytitle\"] ` +
		`path/to/src/directory ` +
		`s3://bucket/destination/file.tar.gz ` +
		`https://platform.silverstripe.com/naut/project/MYPROJECT/environment/MYENV\n`

	_, err := fmt.Fprintf(os.Stderr, format, os.Args[0])
	handleError(err)
}
