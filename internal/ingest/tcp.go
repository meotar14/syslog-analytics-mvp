package ingest

import (
	"bufio"
	"context"
	"io"
	"log"
	"net"
	"strconv"
	"strings"

	"syslog-analytics-mvp/internal/parse"
	"syslog-analytics-mvp/internal/stats"
)

func StartTCP(ctx context.Context, addr string, collector *stats.Collector) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go func() {
		defer listener.Close()
		log.Printf("tcp syslog listener on %s", addr)
		go func() {
			<-ctx.Done()
			_ = listener.Close()
		}()

		for {
			conn, err := listener.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("tcp accept error: %v", err)
				continue
			}
			go handleTCPConn(ctx, conn, collector)
		}
	}()

	return nil
}

func handleTCPConn(ctx context.Context, conn net.Conn, collector *stats.Collector) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	sourceIP, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		sourceIP = conn.RemoteAddr().String()
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		message, err := readTCPMessage(reader)
		if message != "" {
			collector.Record(sourceIP, parse.Parse(message))
		}
		if err != nil {
			if err == io.EOF {
				return
			}
			log.Printf("tcp read error from %s: %v", sourceIP, err)
			return
		}
	}
}

func readTCPMessage(reader *bufio.Reader) (string, error) {
	peeked, err := reader.Peek(1)
	if err != nil {
		return "", err
	}

	if peeked[0] >= '1' && peeked[0] <= '9' {
		if framed, ok, err := tryReadOctetCounted(reader); ok || err != nil {
			return framed, err
		}
	}

	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), err
}

func tryReadOctetCounted(reader *bufio.Reader) (string, bool, error) {
	header, err := reader.Peek(16)
	if err != nil && err != bufio.ErrBufferFull && err != io.EOF {
		return "", false, err
	}

	end := strings.IndexByte(string(header), ' ')
	if end <= 0 {
		return "", false, nil
	}
	lengthPart := string(header[:end])
	size, err := strconv.Atoi(lengthPart)
	if err != nil || size <= 0 {
		return "", false, nil
	}

	if _, err := reader.Discard(end + 1); err != nil {
		return "", true, err
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return "", true, err
	}
	return strings.TrimSpace(string(payload)), true, nil
}
