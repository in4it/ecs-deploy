#
# build angular project
#
FROM node:22.19.0-slim AS webapp-builder

# change PREFIX if you need another url prefix for the webapp
ENV PREFIX=/ecs-deploy

COPY webapp/package.json webapp/package-lock.json ./

RUN npm set progress=false && npm config set depth 0 && npm cache clean --force

RUN npm i && mkdir -p /webapp && mv package.json package-lock.json ./node_modules /webapp

WORKDIR /webapp

COPY webapp /webapp

RUN node_modules/.bin/ng build --configuration production --base-href ${PREFIX}/webapp/

#
# Build go project
#
FROM golang:1.24.7-alpine AS go-builder

WORKDIR /ecs-deploy/

COPY . .

RUN apk add -u -t build-tools curl git && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ecs-deploy cmd/ecs-deploy/main.go && \
    apk del build-tools && \
    rm -rf /var/cache/apk/*

#
# Runtime container
#
FROM alpine:3.22.1

ARG SOURCE_COMMIT=unknown

ENV GIN_MODE=release

RUN apk --no-cache add ca-certificates bash curl && mkdir -p /app/webapp

WORKDIR /app

COPY . .
COPY --from=go-builder /ecs-deploy/ecs-deploy .
COPY --from=webapp-builder /webapp/dist/browser webapp/dist

RUN echo ${SOURCE_COMMIT} > source_commit

# remove unnecessary source files
RUN rm -rf *.go webapp/src

# create a non-root user to run the application
RUN apk add --no-cache shadow \
    && useradd -u 1000 -U -m appuser \
    && chown -R appuser:appuser /app

# Switch to the non-root user
USER appuser

CMD ["./ecs-deploy", "--server"]