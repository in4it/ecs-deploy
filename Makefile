SERVER_BINARY = ecs-deploy
CLIENT_BINARY = ecs-client
GOARCH = amd64

all: deps build

build: build-server build-client

deps:
	dep ensure

build-server:
	GOOS=linux GOARCH=${GOARCH} go build ${LDFLAGS} -o ${SERVER_BINARY}-linux-${GOARCH} cmd/ecs-deploy/main.go 

build-client:
	GOOS=linux GOARCH=${GOARCH} go build ${LDFLAGS} -o ${CLIENT_BINARY}-linux-${GOARCH} cmd/ecs-client/main.go 

test:
	go test

integrationTest:
	export $$(cat .env | grep -v '^\#' | xargs) && go test -timeout 1h -run Integration
	
clean:
	rm -f ${BINARY}-linux-${GOARCH}
