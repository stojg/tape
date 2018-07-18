VERSION=`git describe --tags`
LDFLAGS=-ldflags "-s -w -X main.version=${VERSION}"
BINARY=tape

all:
	mkdir -p ./build
	go build ${LDFLAGS} -o ./build/${BINARY} .

compile:
	mkdir -p ./build
	GOARCH=amd64 GOOS=linux go build -o ./build/${BINARY}_linux ${LDFLAGS} .
	GOARCH=amd64 GOOS=darwin go build -o ./build/${BINARY}_darwin ${LDFLAGS} .
	GOARCH=amd64 GOOS=windows go build -o ./build/${BINARY}_windows ${LDFLAGS} .
	GOARM=7 GOARCH=arm GOOS=linux go build -o ./build/${BINARY}_arm ${LDFLAGS} .

clean:
	if [ -d ./build ] ; then rm -rf ./build ; fi
