# Load Tester

Included in this directory is a sample load_test script.


### Running the script manually

To manually run the script, run `./run.sh`


### Configuring the load test script

Change address of file server
`export FILE_SERVER_ADDR=http://file-server-addr:port`

Change max requests per second the load-test script will attempt to submit to the file server.
`export REQUESTS_PER_SECOND=25`

Change max # of files the load test script will read/write to.
`export MAX_FILE_COUNT=500`

Change max fize size that will be generated (number of bytes)
`export MAX_FILE_SIZE=1024`
