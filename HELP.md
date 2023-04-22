### Commands:

`make help`

### To start the file server application

make `start`

or

`docker-compose up -d`

### To stop the file server application

`make stop`

or

`docker-compose down`


### Rebuild & restart fresh file server and load tester

`make start-clean`

or

`docker-compose stop  && docker-compose rm -f && docker-compose build --no-cache  && docker-compose up -d --remove-orphans`

### Tail logs of load test container

`docker logs -f load-tester`

### View stats of running containers (CPU / Memory)
`make stats`

or 

`docker stats`

### Add a sample file

`curl -i -X PUT http://localhost:1234/api/fileserver/file-name-1 -d "file-contents"`


### Read a sample file

`curl -i http://localhost:1234/api/fileserver/file-name-1`