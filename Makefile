SERVER_BINARY = ecs-deploy
CLIENT_BINARY = ecs-client
GOARCH = amd64

all: build

build: build-server build-client

build-static: build-server-static build-client-static

test: test-main test-client

build-server:
	GOOS=linux GOARCH=${GOARCH} go build ${LDFLAGS} -o ${SERVER_BINARY}-linux-${GOARCH} cmd/ecs-deploy/main.go 

build-server-darwin:
	GOOS=darwin GOARCH=${GOARCH} go build ${LDFLAGS} -o ${SERVER_BINARY}-linux-${GOARCH} cmd/ecs-deploy/main.go 

build-client:
	GOOS=linux GOARCH=${GOARCH} go build ${LDFLAGS} -o ${CLIENT_BINARY}-linux-${GOARCH} cmd/ecs-client/main.go 
build-client-darwin:
	GOOS=darwin GOARCH=${GOARCH} go build ${LDFLAGS} -o ${CLIENT_BINARY}-linux-${GOARCH} cmd/ecs-client/main.go 

build-server-static:
	CGO_ENABLED=0 GOOS=linux GOARCH=${GOARCH} go build -a -installsuffix cgo ${LDFLAGS} -o ${SERVER_BINARY}-linux-${GOARCH} cmd/ecs-deploy/main.go 

build-client-static:
	CGO_ENABLED=0 GOOS=linux GOARCH=${GOARCH} go build -a -installsuffix cgo ${LDFLAGS} -o ${CLIENT_BINARY}-linux-${GOARCH} cmd/ecs-client/main.go 

test-main:
	cd test && go test

test-client:
	cd cmd/ecs-client && go test

test-provider:
	cd provider/ecs && go test

integrationTest:
	cd test && export $$(cat ../.env | grep -v '^\#' | xargs) && go test -timeout 1h -run Integration
	
clean:
	rm -f ${BINARY}-linux-${GOARCH}
