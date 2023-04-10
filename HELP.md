### To start the file server application

`docker-compose up -d`

### To stop the file server application

`docker-compose down`


### Rebuild & restart fresh fileserver and load tester

`docker-compose stop  && docker-compose rm -f && docker-compose build --no-cache  && docker-compose up -d --remove-orphans`

### Tail logs of load test container

`docker logs -f fileserver-challenge-load_tester-1`


### Add a sample file

`curl -i -X PUT http://localhost:1234/api/fileserver/file-name-1 -d "file-contents"`