FROM golang:latest AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGET=./cmd/relay
RUN go build -o /app $TARGET

FROM golang:latest
COPY --from=build /app /app
CMD ["/app"]
