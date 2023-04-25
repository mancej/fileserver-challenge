#!/bin/zsh

SCRIPTPATH="$( cd "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

export FILE_SERVER_HOST=localhost            # Point this to your application middleware
export FILE_SERVER_PORT=8080                   # Point this to your application middleware (port will change)
export FILE_SERVER_PROTO=http
export REQUESTS_PER_SECOND=1                   # Base requests/sec the load test will begin on.
export SEED_GROWTH_AMOUNT=1                    # Every second, this many more requests will be scheduled
export ENABLE_REQUEST_RAMP=true                # If true, every 1 minute, your seed growth rate doubles
export ENABLE_FILE_RAMP=false                  # If true, every 15 seconds the max possible file size written increases by 50%
export RANDOMLY_UPLOAD_LARGE_FILES=true        # If true, 1 out of every 100 files uploaded will be > 100MB in size
export MAX_FILE_COUNT=2500                     # Recommend 2-5x total REQUESTS_PER_SECOND (consider seed in this calculation)
export MAX_FILE_SIZE=1024                      # 1KB, but could be set to ANYTHING in live tests

go run $SCRIPTPATH/../cmd/main.go