# One image with all binaries. Compose picks the binary with "command".

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ ./cmd/...

FROM alpine:3.22
RUN adduser -D -u 10001 sparecycles
COPY --from=build /out/ /usr/local/bin/
USER sparecycles
WORKDIR /home/sparecycles
EXPOSE 7400 9400
CMD ["coordinator"]
