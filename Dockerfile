#
# build angular project
#
FROM node:8.9 as webapp-builder

# change PREFIX if you need another url prefix for the webapp
ENV PREFIX /ecs-deploy

COPY webapp/package.json webapp/package-lock.json ./

RUN npm set progress=false && npm config set depth 0 && npm cache clean --force

RUN npm i && mkdir -p /webapp && mv package.json package-lock.json ./node_modules /webapp

WORKDIR /webapp

COPY webapp /webapp

RUN cd /webapp && $(npm bin)/ng build --prod --base-href ${PREFIX}/webapp/

#
# Build go project
#
FROM golang:1.9 as go-builder

WORKDIR /go/src/github.com/in4it/ecs-deploy/

COPY . .

RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh && dep ensure
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ecs-deploy cmd/ecs-deploy/main.go

#
# Runtime container
#
FROM alpine:latest  

ARG SOURCE_COMMIT=unknown

ENV GIN_MODE release

RUN apk --no-cache add ca-certificates bash curl && mkdir -p /app/webapp

WORKDIR /app

COPY . .
COPY --from=go-builder /go/src/github.com/in4it/ecs-deploy/ecs-deploy .
COPY --from=webapp-builder /webapp/dist webapp/dist

RUN echo ${SOURCE_COMMIT} > source_commit

# remove unnecessary source files
RUN rm -rf *.go webapp/src

CMD ["./ecs-deploy", "--server"]  
