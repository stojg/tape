VERSION=`git describe --tags`
LDFLAGS=-ldflags "-s -w -X main.version=${VERSION}"
BINARY=tape

all: clean
	mkdir -p ./build
	go build ${LDFLAGS} -o ./build/${BINARY} .

clean:
	if [ -d ./build ] ; then rm -rf ./build ; fi

compile: clean
	mkdir -p ./build
	GOARCH=amd64 GOOS=linux go build -o ./build/${BINARY}_linux_${VERSION} ${LDFLAGS} .
	GOARCH=amd64 GOOS=darwin go build -o ./build/${BINARY}_darwin_${VERSION} ${LDFLAGS} .
	GOARCH=amd64 GOOS=windows go build -o ./build/${BINARY}_windows_${VERSION} ${LDFLAGS} .
	GOARM=7 GOARCH=arm GOOS=linux go build -o ./build/${BINARY}_arm_${VERSION} ${LDFLAGS} .
