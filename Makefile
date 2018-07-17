LDFLAGS=-ldflags "-s -w"
BINARY=tape

all:
	go build ${LDFLAGS} -o ${BINARY} .

compile:

	GOARCH=amd64 GOOS=linux go build -o ${BINARY}_linux ${LDFLAGS} .
	GOARCH=amd64 GOOS=darwin go build -o ${BINARY}_darwin ${LDFLAGS} .
	GOARCH=amd64 GOOS=windows go build -o ${BINARY}_windows ${LDFLAGS} .
	GOARM=7 GOARCH=arm GOOS=linux go build -o ${BINARY}_arm ${LDFLAGS} .

clean:
	if [ -f ${BINARY} ] ; then rm ${BINARY} ; fi
	if [ -f ${BINARY}_linux ] ; then rm ${BINARY}_linux ; fi
	if [ -f ${BINARY}_windows ] ; then rm ${BINARY}_windows ; fi
	if [ -f ${BINARY}_darwin ] ; then rm ${BINARY}_darwin ; fi
	if [ -f ${BINARY}_arm ] ; then rm ${BINARY}_arm ; fi
