FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api .

FROM alpine:3.21
RUN addgroup -S app && adduser -S -G app app && apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/api /app/api
USER app
EXPOSE 8081
ENTRYPOINT ["/app/api"]
