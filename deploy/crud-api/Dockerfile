FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /out/crud-api ./cmd/crud-api

FROM alpine:3.20
COPY --from=build /out/crud-api /crud-api
ENV LOG_FILE=/var/log/logpipe/app.log
ENV ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["/crud-api"]
