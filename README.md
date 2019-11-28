# tape

Tape tar:s and gzips a directory and deploys site code to an SilverStripe platform environment.

This software is beta and should only be used in a CI/CD environment to produce reproducible builds.

Note that this software is not endorsed by SilverStripe ltd and can cause site outages and other
severe problems if not used carefully.

## installation

You can grab a version of tape from https://github.com/stojg/tape/releases for your distribution, make
sure that you set any executable bits on it after download.

## Example

`tape --title "my deploy title" path/to/src/directory s3://bucket/destination/file.tgz https://platform.silverstripe.com/naut/project/MYPROJECT/environment/MYENV`

This will immediately deploy and wait until deployment is finished.

It requires deploy access to an environment on the SilverStripe platform and access to write to an AWS S3 bucket.

## Configuration

the following environment variables must be set:

 - `DASHBOARD_TOKEN` - create one at [SilverStripe dashboard](https://platform.silverstripe.com/naut/profile)
 - `DASHBOARD_USER` - the user for the above token, ie your username
 - `AWS_DEFAULT_REGION` - the region for the AWS S3 bucket
 - `AWS_ACCESS_KEY_ID` - the access key for a user allowed to write to the S3 bucket
 - `AWS_SECRET_ACCESS_KEY` - the secret key for a user allowed to write to the S3 bucket

Example codeship test pipeline configuration

```
./vendor/bin/phpunit

# remove dev packages
composer install --no-dev && rm -rf .git

# Step out of source directory
SRC_DIR=`pwd -P` && cd ../

# install tape
curl --silent --show-error --fail --location --output ./tape https://github.com/stojg/tape/releases/download/1.4.0/tape_linux_1.4.0 && chmod +x ./tape

# pack up, schedule and deploy
./tape -title "my deploy title" ${SRC_DIR} s3://bucket/destination/file.tgz https://platform.silverstripe.com/naut/project/MYPROJECT/environment/MYENV
```

## Example output:

```
tape 1.4.0
[-] scanning directory
[-] directory scan completed
[-] 5571 files were compressed into a tar archive
[-] S3 upload completed
[-] requesting pre-signed link to the S3 object
[-] requesting deployment from platform.playpen.pl
[-] starting deployment 3236
[-] deployment currently in state 'Deploying'
[-] deployment currently in state 'Deploying'
[-] deployment currently in state 'Deploying'
[-] deployment currently in state 'Completed'

[=] deployment successful!
4m14.453010001s
```
