FROM golang:1.22-alpine AS build

RUN apk add --no-cache git ca-certificates tzdata
    
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
    
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /out/app/tg-release-bot ./cmd/bot

RUN mkdir -p /out/app/data
RUN mkdir -p /out/etc/ssl/certs /out/usr/share/zoneinfo \
 && cp /etc/ssl/certs/ca-certificates.crt /out/etc/ssl/certs/ \
 && cp -a /usr/share/zoneinfo /out/usr/share/

FROM gcr.io/distroless/static:nonroot

WORKDIR /app

COPY --from=build --chown=65532:65532 /out/app /app
COPY --from=build /out/etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/usr/share/zoneinfo /usr/share/zoneinfo

ENV DB_PATH=/app/data/releases.db
ENV TZ=Europe/Amsterdam

USER 65532:65532

ENTRYPOINT ["/app/tg-release-bot"]
    