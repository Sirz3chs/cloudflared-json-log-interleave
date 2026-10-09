package main

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
)

type countingWriter struct {
	bytes.Buffer
	writes int
}

func (c *countingWriter) Write(p []byte) (int, error) {
	c.writes++
	return c.Buffer.Write(p)
}

func writesPerEvent(newWriter func(*countingWriter) interface{ Write([]byte) (int, error) }) (int, string) {
	w := &countingWriter{}
	logger := zerolog.New(newWriter(w)).With().Timestamp().Logger()
	logger.Error().Int("connIndex", 3).Str("error", "context canceled").Str("type", "http").Msg("Request failed")
	return w.writes, w.String()
}

// Shows the bug: cloudflared's writer splits one event into one Write per field.
func TestOriginalWritesPerEvent(t *testing.T) {
	n, out := writesPerEvent(func(w *countingWriter) interface{ Write([]byte) (int, error) } { return &consoleWriter{out: w} })
	t.Logf("original: %d writes for one event: %s", n, out)
	if n == 1 {
		t.Errorf("expected the original writer to split the event, got a single write")
	}
}

// The proposed fix writes each event in a single Write.
func TestFixedSingleWritePerEvent(t *testing.T) {
	n, out := writesPerEvent(func(w *countingWriter) interface{ Write([]byte) (int, error) } { return &fixedWriter{out: w} })
	if n != 1 {
		t.Errorf("expected 1 write for the log event, got %d: %s", n, out)
	}
}
