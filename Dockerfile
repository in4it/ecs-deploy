FROM golang:1.9

ENV GIN_MODE release

ENV URL_PREFIX /ecs-deploy

WORKDIR /go/src/app
COPY . .

RUN go-wrapper download   
RUN go-wrapper install  

CMD ["go-wrapper", "run"]
