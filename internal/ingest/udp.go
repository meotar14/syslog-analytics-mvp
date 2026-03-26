package ingest

import (
	"context"
	"log"
	"net"

	"syslog-analytics-mvp/internal/parse"
	"syslog-analytics-mvp/internal/stats"
)

func StartUDP(ctx context.Context, addr string, collector *stats.Collector) error {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return err
	}

	go func() {
		defer conn.Close()
		log.Printf("udp syslog listener on %s", addr)
		buffer := make([]byte, 64*1024)

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			n, remote, err := conn.ReadFrom(buffer)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("udp read error: %v", err)
				continue
			}

			parsed := parse.Parse(string(buffer[:n]))
			host, _, splitErr := net.SplitHostPort(remote.String())
			if splitErr != nil {
				host = remote.String()
			}
			collector.Record(host, parsed)
		}
	}()
	return nil
}
