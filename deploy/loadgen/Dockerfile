FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /out/loadgen ./cmd/loadgen

FROM alpine:3.20
COPY --from=build /out/loadgen /loadgen
ENTRYPOINT ["/loadgen"]
