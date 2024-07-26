#!/bin/bash

EXAMPLE=$1
JOB_DIR=/tmp/jobs/${EXAMPLE}

cd ${JOB_DIR}; cat sample.csv | ./throttle --milliseconds 300 --append-timestamp false | ./grizzly -p ./plan.bin -x 3600 2>> ./grizzly.log
