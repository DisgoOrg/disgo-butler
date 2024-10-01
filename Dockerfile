FROM golang:1.23-alpine AS build

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG VERSION=dev

RUN CGO_ENABLED=0 go build -ldflags="-X 'main.version=${VERSION}'-w -s" -o bot main.go

FROM alpine

COPY --from=build /build/bot /bin/bot

ENTRYPOINT ["/bin/bot"]
