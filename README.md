# tape

Tape tar:s and gzips a directory and deploys site code to an Silverstripe cloud environment.

This software is beta and should only be used in a CI/CD environment to produce reproducible builds.

Note that this software is not endorsed by Silverstripe LTD and can cause site outages and other
severe problems if not used carefully.

## Prerequisite

tape is using AWS s3 buckets as an intermediate storage for your source code package and hence requires that you have 
access credentials to an S3 bucket location with write access and requesting pre-signed read links for objects.

## Installation

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

Example [codeship.com](https://codeship.com/) test pipeline configuration

```
./vendor/bin/phpunit

# remove dev packages
composer install --no-progress --prefer-dist --no-dev --ignore-platform-reqs --optimize-autoloader --no-interaction --no-suggest && rm -rf .git

# Step out of source directory
SRC_DIR=`pwd -P` && cd ../

# install tape
curl --silent --show-error --fail --location --output ./tape https://github.com/stojg/tape/releases/download/1.5.0/tape_linux_1.5.0 && chmod +x ./tape

# pack up, schedule and deploy
./tape -verbose -title "my deploy title" ${SRC_DIR} s3://bucket/destination/file.tgz https://platform.silverstripe.com/naut/project/MYPROJECT/environment/MYENV
```

## Example output:

```
tape 1.5.0
[-] starting upload to s3://public.stojg.se/sandbox/bledge.tgz
[-] S3 upload completed
[-] requesting a 5 minute pre-signed link to the S3 object
[-] requesting deployment from platform.playpen.pl
[-] starting deployment 4539
[-] deployment currently in state 'Deploying'
[-] deployment currently in state 'Deploying'
[-] deployment currently in state 'Completed'
[-] 
[+] deployment successful! 🍺
```
