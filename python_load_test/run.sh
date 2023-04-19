#!/bin/sh
scriptDir="$( cd "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

## Install requirements && run load_test. Your fileserver stack must be running.
export FILE_SERVER_ADDR="http://localhost:1234"
export REQUESTS_PER_SECOND=20
export MAX_FILE_COUNT=500
export MAX_FILE_SIZE=1024

# export LOG_LEVEL=DEBUG # To enable debug logs, uncomment this.

## You must have python 3.9+ installed. (tested with 3.9)
python -m ensurepip --upgrade
pip install --upgrade pip
pip install -r ${scriptDir}/requirements.txt
python ${scriptDir}/main.py