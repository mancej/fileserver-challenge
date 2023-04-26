#!/bin/zsh

curl http://localhost:8080/api/fileserver/huge-file --upload-file /tmp/hugefile
