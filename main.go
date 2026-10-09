// Reproducer for interleaved JSON log lines in cloudflared --output json.
//
// consoleWriter below is a verbatim copy of cloudflared's logger/console.go
// (unchanged since TUN-9371, 2025.6). fixedWriter is the proposed fix.
//
// Usage: go run . [original|fixed] 2>out.log
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"

	jsoniter "github.com/json-iterator/go"
	"github.com/rs/zerolog"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// consoleWriter is a verbatim copy of cloudflared logger/console.go.
type consoleWriter struct {
	out io.Writer
}

func (c *consoleWriter) Write(p []byte) (n int, err error) {
	var evt map[string]any
	d := json.NewDecoder(bytes.NewReader(p))
	d.UseNumber()
	err = d.Decode(&evt)
	if err != nil {
		return n, fmt.Errorf("cannot decode event: %s", err)
	}
	e := json.NewEncoder(c.out)
	return len(p), e.Encode(evt)
}

// fixedWriter encodes the event into a buffer and writes it once.
type fixedWriter struct {
	out io.Writer
}

func (c *fixedWriter) Write(p []byte) (n int, err error) {
	var evt map[string]any
	d := json.NewDecoder(bytes.NewReader(p))
	d.UseNumber()
	err = d.Decode(&evt)
	if err != nil {
		return n, fmt.Errorf("cannot decode event: %s", err)
	}
	var buf bytes.Buffer
	if err = json.NewEncoder(&buf).Encode(evt); err != nil {
		return n, err
	}
	_, err = c.out.Write(buf.Bytes())
	return len(p), err
}

const (
	goroutines = 8
	events     = 5000
)

func main() {
	mode := "original"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	var w io.Writer
	switch mode {
	case "original":
		w = &consoleWriter{out: os.Stderr}
	case "fixed":
		w = &fixedWriter{out: os.Stderr}
	default:
		fmt.Fprintf(os.Stdout, "unknown mode %q, want original or fixed\n", mode)
		os.Exit(2)
	}

	// Same shape as cloudflared's "Request failed" events during a burst of
	// cancelled requests.
	log := zerolog.New(w).With().Timestamp().Logger()
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < events; i++ {
				log.Error().
					Int("connIndex", g).
					Int("event", 0).
					Str("dest", "https://example.com/api/x").
					Str("error", "Incoming request ended abruptly: context canceled").
					Str("type", "http").
					Msg("Request failed")
			}
		}(g)
	}
	wg.Wait()
}
