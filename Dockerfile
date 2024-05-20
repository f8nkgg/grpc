FROM golang:latest as modules

WORKDIR /modules
COPY go.mod go.sum ./
RUN go mod download


FROM golang:latest as builder
WORKDIR /app
COPY --from=modules /go/pkg /go/pkg
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/app ./cmd/app


FROM alpine:latest
WORKDIR /app
COPY --from=builder /bin/app ./
CMD ["./app"]