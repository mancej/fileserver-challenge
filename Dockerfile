FROM golang:buster as build

ENV CGO_ENABLED=0

WORKDIR /build

COPY . .

RUN go build -o main cmd/main.go

FROM scratch

COPY --from=build build/main /go/bin/main

ENTRYPOINT [ "/go/bin/main" ]

