#!/bin/bash
#
# local server
#
# create a .env file with all the variables first,
# then execute this script using ./examples/localserver.sh

eval $(egrep -v '^#' .env | xargs) go run *.go
