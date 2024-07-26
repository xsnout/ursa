#!/bin/bash

CONTAINER=$(docker ps -a -q --filter="name=ursa-ursa")
docker stop $CONTAINER || true
docker rm $CONTAINER || true
docker rmi $(docker images -a -q "ursa-ursa") || true
