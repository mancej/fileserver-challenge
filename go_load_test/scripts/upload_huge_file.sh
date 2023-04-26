#!/bin/zsh

curl -v -X PUT http://localhost:8080/api/fileserver/huge-file --data @/tmp/hugefile