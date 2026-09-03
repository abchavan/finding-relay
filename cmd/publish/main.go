// Command publish sends an NDJSON file of findings to JetStream.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/nats-io/nats.go"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "publish:", err)
		os.Exit(1)
	}
}

func run() error {
	file := flag.String("file", "dev/samples/findings.ndjson", "NDJSON file to publish")
	limit := flag.Int("n", 0, "number of lines to publish (0 = all)")
	url := flag.String("nats", "nats://localhost:4222", "NATS server URL")
	user := flag.String("user", "", "NATS user")
	pass := flag.String("password", "", "NATS password")
	prefix := flag.String("prefix", "findings", "subject prefix; the scanner name is appended")
	flag.Parse()

	nc, err := nats.Connect(*url, nats.UserInfo(*user, *pass))
	if err != nil {
		return err
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		return err
	}

	f, err := os.Open(*file)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	count := 0
	for sc.Scan() && (*limit == 0 || count < *limit) {
		line := append([]byte(nil), sc.Bytes()...)
		if len(line) == 0 {
			continue
		}
		var head struct {
			ID      string `json:"id"`
			Scanner string `json:"scanner"`
		}
		if err := json.Unmarshal(line, &head); err != nil {
			return fmt.Errorf("line %d: %w", count+1, err)
		}
		msg := nats.NewMsg(*prefix + "." + head.Scanner)
		msg.Header.Set(nats.MsgIdHdr, head.ID)
		msg.Data = line
		if _, err := js.PublishMsg(msg); err != nil {
			return fmt.Errorf("line %d: %w", count+1, err)
		}
		count++
	}
	if err := sc.Err(); err != nil {
		return err
	}
	fmt.Printf("published %d\n", count)
	return nil
}
