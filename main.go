package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/silverstripeltd/ssp-sdk-go/ssp"
)

// @todo output how long this deployment might take (fast|full)
// @todo allow deploy = true|false argument

var (
	version string
)

type FileStat struct {
	baseDir      string
	relativePath string
	link         string
	stat         os.FileInfo
	err          error
}

func main() {

	fmt.Printf("tape %s\n", version)

	title := flag.String("title", "[CI] Deploy", "A title to use in the deployments title")
	verbose := flag.Bool("verbose", false, "verbose output")
	flag.Parse()

	if len(flag.Args()) != 3 {
		usage()
		os.Exit(1)
	}

	handleError(realMain(*title, *verbose))
}

func realMain(title string, verbose bool) error {
	startTime := time.Now()

	app := newApp(flag.Args(), verbose)

	files, err := scanDirectory(app)
	if err != nil {
		return err
	}

	// remove .git etc
	filteredFiles := filterFiles(app, files)

	// use a pipe between the tar compressing and S3 uploading so that we don't have to waste disk space
	reader, writer := io.Pipe()

	// spin off into a go routine so that the upload can stream the output from this into the uploader via the writer
	go func() {
		err := buildPackage(app, writer, filteredFiles)
		handleError(err)
	}()

	if err := upload(app, reader); err != nil {
		return err
	}

	preSignedURL, err := preSignedLink(app)
	if err != nil {
		return err
	}

	dep, err := createDeployment(app, preSignedURL, title)
	if err != nil {
		return err
	}

	dep, err = startDeployment(app, dep)
	if err != nil {
		return err
	}

	if _, err := waitForDeployResult(app, dep); err != nil {
		return err
	}

	app.Infof("\n[=] deployment successful! 🍺")
	app.Infof("    %s\n", time.Since(startTime))
	return nil
}

func filterFiles(app application, files <-chan FileStat) <-chan FileStat {
	out := make(chan FileStat)

	go func() {
		var filtered uint64
		for file := range files {
			if file.relativePath == "." {
				continue
			}
			if file.stat.Name() == ".git" && file.stat.IsDir() {
				filtered++
				continue
			}
			if strings.HasPrefix(file.relativePath, ".git/") {
				filtered++
				continue
			}
			if strings.Contains(file.relativePath, "/.git/") {
				filtered++
				continue
			}
			out <- file
		}
		app.Debugf("excluded %d files (.git etc)\n", filtered)
		close(out)
	}()
	return out
}

func upload(app application, source io.ReadCloser) error {
	// Create an uploader (can do multipart) with S3 client and default options
	uploader := s3manager.NewUploaderWithClient(app.S3Client())

	r := NewSizeReader(source)
	params := &s3manager.UploadInput{
		Bucket:      aws.String(app.s3.bucket),
		Key:         aws.String(app.s3.key),
		ContentType: aws.String("application/x-tgz"),
		Body:        r,
	}

	app.Infof("starting upload to s3://%s\n", path.Join(app.s3.bucket, app.s3.key))

	timer := time.NewTicker(1 * time.Second)
	go func(tick *time.Ticker) {
		for {
			select {
			case <-tick.C:
				n, done := r.N()
				if done {
					app.Debugf("%s of data is being uploaded", byteCountDecimal(n))
					tick.Stop()
					return
				}
			}
		}
	}(timer)

	start := time.Now()
	_, err := uploader.Upload(params)

	closer(r)
	// try to parse AWS errors so they are not so butt ugly to display
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			switch awsErr.Code() {
			case "AccessDenied":
				return fmt.Errorf("upload to S3 bucket '%s' failed, got an access denied error", app.s3.bucket)
			case "NoCredentialProviders":
				return fmt.Errorf("upload failed: tape could not find AWS credentials to use")
			default:
				return fmt.Errorf("upload failed: %s - %s", awsErr.Code(), awsErr.Message())
			}
		}
		return err
	}

	app.Infof("S3 upload completed")
	app.Debugf("Upload completed in %s", time.Since(start))
	return nil
}

func preSignedLink(app application) (string, error) {
	app.Infof("requesting a 5 minute pre-signed link to the S3 object")
	req, _ := app.S3Client().GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(app.s3.bucket),
		Key:    aws.String(app.s3.key),
	})
	return req.Presign(300 * time.Second)
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

func createDeployment(app application, packageURL, title string) (*ssp.Deployment, error) {
	app.Infof("requesting deployment from %s\n", app.dashboard.url.Host)
	client, err := app.dashboardClient()
	if err != nil {
		return nil, err
	}

	dep, err := client.CreateDeployment(app.stack, app.env, &ssp.CreateDeployment{
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

func startDeployment(app application, d *ssp.Deployment) (*ssp.Deployment, error) {
	app.Infof("starting deployment %d\n", d.ID)

	client, err := app.dashboardClient()
	if err != nil {
		return nil, err
	}
	req := &ssp.StartDeployment{ID: d.ID}
	d, err = client.StartDeployment(d.Stack.ID, d.Environment.ID, req)
	return d, err
}

func waitForDeployResult(app application, d *ssp.Deployment) (*ssp.Deployment, error) {
	client, err := app.dashboardClient()
	if err != nil {
		return nil, err
	}

	var state ssp.State = ssp.StateNew

	progressTick := time.NewTicker(time.Minute)
	checkTick := time.NewTicker(time.Second * 5)
	cancelTick := time.NewTimer(time.Minute * 25)

	deployURL := path.Join(app.dashboard.url.String(), "overview", "deployment", strconv.Itoa(d.ID))

	for {
		select {
		// need to output something so that CodeShip doesn't cancel the build due to no output
		case <-progressTick.C:
			app.Infof("[-] deployment currently in state '%s'\n", d.State)

		case <-checkTick.C:
			d, err = client.GetDeployment(d.Stack.ID, d.Environment.ID, fmt.Sprintf("%d", d.ID))
			if err != nil {
				return d, err
			}

			if d.State != state {
				// only display state changes
				app.Infof("[-] deployment currently in state '%s'\n", d.State)
				state = d.State
			}

			if d.State == ssp.StateFailed {
				return d, fmt.Errorf("deployment failed, check logs at %s\n", deployURL)
			}
			if d.State == ssp.StateCompleted {
				return d, nil
			}

		case <-cancelTick.C:
			return d, fmt.Errorf("waiting for deployment to finish timed out, check logs at %s\n", deployURL)
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

func closer(x io.Closer) {
	if err := x.Close(); err != nil {
		handleError(err)
	}
}
