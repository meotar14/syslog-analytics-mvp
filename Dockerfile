FROM golang:1.24-alpine AS build
WORKDIR /src

COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /out/syslog-analytics ./cmd/syslog-analytics

FROM alpine:3.21
RUN adduser -D -u 10001 appuser && mkdir -p /data && chown -R appuser:appuser /data
USER appuser
WORKDIR /app
COPY --from=build /out/syslog-analytics /app/syslog-analytics

EXPOSE 5514/udp
EXPOSE 8080

ENV DB_PATH=/data/syslog-analytics.db
ENV HTTP_LISTEN_ADDR=:8080
ENV UDP_LISTEN_ADDR=:5514

CMD ["/app/syslog-analytics"]
