#!/bin/bash

##
## Example: docker-image-login.sh ursa-base:0.1
##

docker exec -it $(docker container ls --all | grep -w ursa-base:0.1 | awk '{print $1}') bash
