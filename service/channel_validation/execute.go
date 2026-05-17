package channel_validation

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// performRequest issues one POST to the target URL with the given body and
// returns a populated StepOutcome. Handles both SSE and JSON responses
// (Anthropic always returns SSE when stream=true, but a misconfigured proxy
// may return JSON; we degrade gracefully).
//
// Network/transport errors set Status=0, OK=false, ErrorText non-empty.
func performRequest(
	ctx context.Context,
	client *http.Client,
	targetURL string,
	apiKey string,
	body []byte,
	kind StepKind,
) StepOutcome {
	out := StepOutcome{Kind: kind}
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		out.ErrorText = "build request: " + err.Error()
		return out
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Anthropic-Version", "2023-06-01")
	if apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
	}

	resp, err := client.Do(req)
	out.DurationMs = time.Since(start).Milliseconds()
	if err != nil {
		out.ErrorText = "http request: " + err.Error()
		return out
	}
	defer resp.Body.Close()

	out.Status = resp.StatusCode
	out.ContentType = resp.Header.Get("Content-Type")
	out.IsSSE = strings.Contains(strings.ToLower(out.ContentType), "event-stream")
	out.ResponseHeaders = collectResponseHeaders(resp.Header)

	limited := &io.LimitedReader{R: resp.Body, N: MaxBodyBytes + 1}

	var rawBuf bytes.Buffer
	rawBuf.Grow(MaxRawExcerptBytes)
	tee := io.TeeReader(limited, &cappedWriter{dst: &rawBuf, cap: MaxRawExcerptBytes})

	if out.IsSSE {
		acc := newSseAccumulator(&out)
		if err := readSSE(tee, acc); err != nil {
			out.StreamReadError = err.Error()
		}
		acc.finalise()
	} else {
		// JSON or unknown: read everything, attempt JSON parse, fall back to
		// excerpt-only for the UI.
		bodyBytes, err := io.ReadAll(tee)
		if err != nil {
			out.StreamReadError = err.Error()
		}
		// If it accidentally is SSE-shaped despite content-type, try one more
		// parse pass; many lookalikes send `data: ...` without setting the
		// event-stream Content-Type.
		if looksLikeSSE(bodyBytes) {
			out.IsSSE = true
			acc := newSseAccumulator(&out)
			_ = readSSE(bytes.NewReader(bodyBytes), acc)
			acc.finalise()
		}
	}

	out.RawExcerpt = rawBuf.String()
	out.DurationMs = time.Since(start).Milliseconds()
	out.OK = computeStepOK(&out)
	if !out.OK && out.ErrorText == "" {
		out.ErrorText = inferErrorReason(&out)
	}
	return out
}

// readSSE parses Anthropic-format Server-Sent Events from r. Each event is
// dispatched to acc once its `data:` lines accumulate and a blank line
// terminates the event. Returns the first read error encountered.
func readSSE(r io.Reader, acc *sseAccumulator) error {
	scanner := bufio.NewScanner(r)
	// Anthropic events can be large (signed thinking blocks). Lift the
	// per-line limit to 1 MiB; we cap total body size separately.
	scanner.Buffer(make([]byte, 64*1024), 1<<20)

	var dataBuf bytes.Buffer
	flush := func() {
		if dataBuf.Len() > 0 {
			acc.dispatch(bytes.TrimRight(dataBuf.Bytes(), "\n"))
			dataBuf.Reset()
		}
	}

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			flush()
			continue
		}
		if bytes.HasPrefix(line, []byte("data:")) {
			payload := bytes.TrimPrefix(line, []byte("data:"))
			payload = bytes.TrimPrefix(payload, []byte(" "))
			// "[DONE]" sentinel: ignore, but keep collecting in case more events follow.
			if bytes.Equal(bytes.TrimSpace(payload), []byte("[DONE]")) {
				continue
			}
			if dataBuf.Len() > 0 {
				dataBuf.WriteByte('\n')
			}
			dataBuf.Write(payload)
		}
		// "event:" lines are informational only — Anthropic also embeds the
		// type inside the data payload, which is what we dispatch on.
	}
	flush()
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

// looksLikeSSE checks the first ~256 bytes for telltale `data:` prefixes.
// Used to handle upstreams that stream SSE without proper Content-Type.
func looksLikeSSE(b []byte) bool {
	head := b
	if len(head) > 256 {
		head = head[:256]
	}
	return bytes.Contains(head, []byte("data:"))
}

// computeStepOK applies the "did this step succeed" rule: HTTP 2xx, no SSE
// error event, message_delta seen (only required for SSE responses).
func computeStepOK(o *StepOutcome) bool {
	if o.Status < 200 || o.Status >= 300 {
		return false
	}
	if o.SseErrorEventSeen {
		return false
	}
	if o.StreamReadError != "" {
		return false
	}
	if o.IsSSE && !o.MessageDeltaSeen {
		return false
	}
	return true
}

func inferErrorReason(o *StepOutcome) string {
	if o.SseErrorEventMessage != "" {
		return o.SseErrorEventMessage
	}
	if o.StreamReadError != "" {
		return "stream read error: " + o.StreamReadError
	}
	if o.Status > 0 && (o.Status < 200 || o.Status >= 300) {
		// Surface a snippet of the raw excerpt for context.
		excerpt := strings.TrimSpace(o.RawExcerpt)
		if len(excerpt) > 300 {
			excerpt = excerpt[:300] + "..."
		}
		if excerpt != "" {
			return fmt.Sprintf("HTTP %d: %s", o.Status, excerpt)
		}
		return fmt.Sprintf("HTTP %d", o.Status)
	}
	return ""
}

// cappedWriter is a Writer that drops bytes beyond a configured cap. Used to
// build the raw_excerpt without unbounded buffering.
type cappedWriter struct {
	dst *bytes.Buffer
	cap int
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	remaining := w.cap - w.dst.Len()
	if remaining <= 0 {
		return len(p), nil
	}
	if len(p) <= remaining {
		w.dst.Write(p)
		return len(p), nil
	}
	w.dst.Write(p[:remaining])
	return len(p), nil
}

// collectResponseHeaders masks sensitive headers and returns a compact map
// for the UI. Anthropic-side headers (request-id, ratelimit, etc.) carry
// useful debugging context; we deliberately preserve them.
func collectResponseHeaders(h http.Header) map[string]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) == 0 {
			continue
		}
		lower := strings.ToLower(k)
		switch lower {
		case "set-cookie", "x-api-key", "authorization", "cookie":
			out[lower] = "[masked]"
		default:
			val := v[0]
			if len(val) > 512 {
				val = val[:512] + "..."
			}
			out[lower] = val
		}
	}
	return out
}

// buildTargetURL strips trailing slashes from base and appends the messages
// path. Channels can store base URLs with or without trailing slash; both work.
func buildTargetURL(base string) string {
	b := strings.TrimRight(base, "/")
	if b == "" {
		b = "https://api.anthropic.com"
	}
	return b + "/v1/messages"
}

// newHTTPClient returns a Client tuned for validation: 90s per-request
// timeout (longer than the suite's own per-step timeout, so the suite
// timeout fires first and produces a more specific error).
func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 90 * time.Second}
}
