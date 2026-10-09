# cloudflared `--output json` interleaved log lines

Minimal reproducer for invalid JSON log lines written by
[cloudflared](https://github.com/cloudflare/cloudflared) with `--output json`
when several goroutines log at the same time.

## Cause

`logger/console.go` re-encodes every zerolog event with
`jsoniter.ConfigCompatibleWithStandardLibrary.NewEncoder(os.Stderr)`. That
config sorts map keys, and its sorted-map encoder calls `Stream.Write` for each
key. In json-iterator v1.1.12, `Stream.Write` writes straight through to the
underlying writer, so one event becomes one `write(2)` per field, plus one for
`}\n`, with no lock. Concurrent events interleave at field boundaries:

```
,"dest":"https://example.com/api/x","error":"Incoming request ended abruptly: context canceled"{"connIndex":1,"dest":"https://example.com/api/x",…,"type":"http"}
,"event":0,"level":"error","message":"Request failed","time":"2026-10-09T07:56:34+02:00","type":"http"}
}
```

The text output (`zerolog.ConsoleWriter`) builds the whole line before writing
it, so it is not affected.

## Run

Requires Go and `jq`.

```console
$ ./check.sh
original  40000 lines,   6805 invalid JSON
fixed     40000 lines,      0 invalid JSON
```

The count varies between runs. `go test -v` shows the number of writes for a
single event (7 with the original writer, 1 with the fix). The same results run
in [CI](../../actions).

- `consoleWriter` in `main.go` is a verbatim copy of cloudflared's
  `logger/console.go`, with the same `json-iterator` and `zerolog` versions as
  cloudflared's `go.mod`.
- `fixedWriter` is the proposed fix: encode into a buffer, write once.

## Proposed fix

```diff
--- a/logger/console.go
+++ b/logger/console.go
@@ func (c *consoleWriter) Write(p []byte) (n int, err error) {
 		return n, fmt.Errorf("cannot decode event: %s", err)
 	}
-	e := json.NewEncoder(c.out)
-	return len(p), e.Encode(evt)
+	var buf bytes.Buffer
+	if err = json.NewEncoder(&buf).Encode(evt); err != nil {
+		return n, err
+	}
+	_, err = c.out.Write(buf.Bytes())
+	return len(p), err
 }
```
