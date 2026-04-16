package adapteronebot

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	eventmodel "github.com/ohmyopencode/bot-platform/packages/event-model"
)

type timeoutTransportError struct{}

func (timeoutTransportError) Error() string   { return "transport timeout" }
func (timeoutTransportError) Timeout() bool   { return true }
func (timeoutTransportError) Temporary() bool { return true }

func TestOneBotReplySenderMapsReplyTextToSendMsg(t *testing.T) {
	t.Parallel()

	var captured OneBotSendRequest
	sender := NewOneBotReplySender(func(request OneBotSendRequest) (OneBotSendResponse, error) {
		captured = request
		return OneBotSendResponse{MessageID: 2001}, nil
	})

	result, err := sender.ReplyText(eventmodel.ReplyHandle{
		Capability: "onebot.reply",
		TargetID:   "group-42",
		Metadata: map[string]any{
			"message_type": "group",
			"group_id":     int64(42),
		},
	}, "pong")
	if err != nil {
		t.Fatalf("reply text: %v", err)
	}
	if captured.Action != "send_msg" || captured.Params["group_id"] != int64(42) || captured.Params["message"] != "pong" {
		t.Fatalf("unexpected request mapping: %+v", captured)
	}
	if result.MessageID != "msg-2001" {
		t.Fatalf("unexpected reply result: %+v", result)
	}
}

func TestOneBotReplySenderSupportsImageAndFile(t *testing.T) {
	t.Parallel()

	requests := make([]OneBotSendRequest, 0, 2)
	sender := NewOneBotReplySender(func(request OneBotSendRequest) (OneBotSendResponse, error) {
		requests = append(requests, request)
		return OneBotSendResponse{MessageID: 3001}, nil
	})

	handle := eventmodel.ReplyHandle{
		Capability: "onebot.reply",
		TargetID:   "private-10001",
		Metadata: map[string]any{
			"message_type": "private",
			"user_id":      int64(10001),
		},
	}

	if _, err := sender.ReplyImage(handle, "https://example.com/a.png"); err != nil {
		t.Fatalf("reply image: %v", err)
	}
	if _, err := sender.ReplyFile(handle, "https://example.com/file.txt"); err != nil {
		t.Fatalf("reply file: %v", err)
	}

	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}
}

func TestOneBotReplySenderReturnsTransportError(t *testing.T) {
	t.Parallel()

	sender := NewOneBotReplySender(func(request OneBotSendRequest) (OneBotSendResponse, error) {
		return OneBotSendResponse{}, errors.New("send failed")
	})

	_, err := sender.ReplyText(eventmodel.ReplyHandle{
		Capability: "onebot.reply",
		TargetID:   "private-10001",
		Metadata: map[string]any{
			"message_type": "private",
			"user_id":      int64(10001),
		},
	}, "pong")
	if err == nil {
		t.Fatal("expected transport error to bubble up")
	}
}

func TestOneBotReplySenderPreservesTransportTimeoutError(t *testing.T) {
	t.Parallel()

	sender := NewOneBotReplySender(func(request OneBotSendRequest) (OneBotSendResponse, error) {
		return OneBotSendResponse{}, timeoutTransportError{}
	})

	_, err := sender.ReplyText(eventmodel.ReplyHandle{
		Capability: "onebot.reply",
		TargetID:   "private-10001",
		Metadata: map[string]any{
			"message_type": "private",
			"user_id":      int64(10001),
		},
	}, "pong")
	if err == nil {
		t.Fatal("expected transport timeout to bubble up")
	}
	var timeoutErr interface{ Timeout() bool }
	if !errors.As(err, &timeoutErr) || !timeoutErr.Timeout() {
		t.Fatalf("expected timeout-capable error, got %T (%v)", err, err)
	}
}

func TestOneBotReplySenderPreservesTransportRateLimitError(t *testing.T) {
	t.Parallel()

	rateLimitErr := errors.New("onebot transport 429 too many requests")
	sender := NewOneBotReplySender(func(request OneBotSendRequest) (OneBotSendResponse, error) {
		return OneBotSendResponse{}, rateLimitErr
	})

	_, err := sender.ReplyText(eventmodel.ReplyHandle{
		Capability: "onebot.reply",
		TargetID:   "group-42",
		Metadata: map[string]any{
			"message_type": "group",
			"group_id":     int64(42),
		},
	}, "pong")
	if !errors.Is(err, rateLimitErr) {
		t.Fatalf("expected rate-limit error to bubble up unchanged, got %v", err)
	}
	if !strings.Contains(err.Error(), "429") {
		t.Fatalf("expected rate-limit error to retain 429 detail, got %v", err)
	}
}

func TestOneBotReplyHTTPTransportSendsJSONAndReturnsMessageID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("expected application/json content-type, got %q", got)
		}
		var request OneBotSendRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Action != "send_msg" {
			t.Fatalf("expected action send_msg, got %q", request.Action)
		}
		if request.Params["group_id"] != float64(42) {
			t.Fatalf("expected group_id 42, got %+v", request.Params)
		}
		if request.Params["message"] != "pong" {
			t.Fatalf("expected message pong, got %+v", request.Params)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message_id": 4001}`))
	}))
	defer server.Close()

	sender := NewOneBotReplySender(NewOneBotReplyHTTPTransport(server.URL, server.Client()))

	result, err := sender.ReplyText(eventmodel.ReplyHandle{
		Capability: "onebot.reply",
		TargetID:   "group-42",
		Metadata: map[string]any{
			"message_type": "group",
			"group_id":     int64(42),
		},
	}, "pong")
	if err != nil {
		t.Fatalf("reply text over http: %v", err)
	}
	if result.MessageID != "msg-4001" {
		t.Fatalf("expected msg-4001, got %+v", result)
	}
}

func TestOneBotReplyHTTPTransportClassifiesTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(100 * time.Millisecond):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"message_id": 4002}`))
		case <-r.Context().Done():
		}
	}))
	defer server.Close()

	client := server.Client()
	client.Timeout = 20 * time.Millisecond
	sender := NewOneBotReplySender(NewOneBotReplyHTTPTransport(server.URL, client))

	_, err := sender.ReplyText(eventmodel.ReplyHandle{
		Capability: "onebot.reply",
		TargetID:   "private-10001",
		Metadata: map[string]any{
			"message_type": "private",
			"user_id":      int64(10001),
		},
	}, "pong")
	if err == nil {
		t.Fatal("expected timeout error")
	}

	var httpErr *OneBotReplyHTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected OneBotReplyHTTPError, got %T (%v)", err, err)
	}
	if !httpErr.Timeout() || !httpErr.TimedOut {
		t.Fatalf("expected timed out classification, got %+v", httpErr)
	}
	if httpErr.RateLimited() {
		t.Fatalf("expected timeout to remain distinct from rate limit, got %+v", httpErr)
	}
	if httpErr.GenericUpstreamFailure() {
		t.Fatalf("expected timeout to remain distinct from generic upstream http status failures, got %+v", httpErr)
	}
	if httpErr.UpstreamFailure() {
		t.Fatalf("expected timeout to remain distinct from upstream http status failures, got %+v", httpErr)
	}
	if httpErr.StatusCode != 0 {
		t.Fatalf("expected no upstream status for client timeout, got %+v", httpErr)
	}
}

func TestOneBotReplyHTTPTransportClassifiesRateLimit(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("slow down"))
	}))
	defer server.Close()

	sender := NewOneBotReplySender(NewOneBotReplyHTTPTransport(server.URL, server.Client()))

	_, err := sender.ReplyText(eventmodel.ReplyHandle{
		Capability: "onebot.reply",
		TargetID:   "group-42",
		Metadata: map[string]any{
			"message_type": "group",
			"group_id":     int64(42),
		},
	}, "pong")
	if err == nil {
		t.Fatal("expected rate-limit error")
	}

	var httpErr *OneBotReplyHTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected OneBotReplyHTTPError, got %T (%v)", err, err)
	}
	if httpErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %+v", httpErr)
	}
	if httpErr.Status != "429 Too Many Requests" {
		t.Fatalf("expected http status text, got %+v", httpErr)
	}
	if httpErr.Body != "slow down" {
		t.Fatalf("expected rate-limit body, got %+v", httpErr)
	}
	if httpErr.RetryAfter != "120" {
		t.Fatalf("expected retry-after hint to be preserved, got %+v", httpErr)
	}
	if httpErr.Timeout() {
		t.Fatalf("expected non-timeout 429 error, got %+v", httpErr)
	}
	if httpErr.GenericUpstreamFailure() {
		t.Fatalf("expected 429 to remain distinct from generic upstream failure, got %+v", httpErr)
	}
	if !httpErr.RateLimited() {
		t.Fatalf("expected 429 to retain rate-limit classification, got %+v", httpErr)
	}
	if !httpErr.UpstreamFailure() {
		t.Fatalf("expected 429 to retain upstream http failure classification, got %+v", httpErr)
	}
	if !strings.Contains(err.Error(), "429") {
		t.Fatalf("expected error string to retain 429 detail, got %v", err)
	}
	if !strings.Contains(err.Error(), "retry-after: 120") {
		t.Fatalf("expected error string to expose retry-after hint, got %v", err)
	}
}

func TestOneBotReplyHTTPTransportClassifiesRateLimitWithoutRetryAfterHint(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("slow down"))
	}))
	defer server.Close()

	sender := NewOneBotReplySender(NewOneBotReplyHTTPTransport(server.URL, server.Client()))

	_, err := sender.ReplyText(eventmodel.ReplyHandle{
		Capability: "onebot.reply",
		TargetID:   "group-42",
		Metadata: map[string]any{
			"message_type": "group",
			"group_id":     int64(42),
		},
	}, "pong")
	if err == nil {
		t.Fatal("expected rate-limit error")
	}

	var httpErr *OneBotReplyHTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected OneBotReplyHTTPError, got %T (%v)", err, err)
	}
	if !httpErr.RateLimited() {
		t.Fatalf("expected 429 to retain rate-limit classification, got %+v", httpErr)
	}
	if httpErr.RetryAfter != "" {
		t.Fatalf("expected missing retry-after hint to remain empty, got %+v", httpErr)
	}
	if strings.Contains(err.Error(), "retry-after:") {
		t.Fatalf("expected error string to stay compatible when retry-after hint is absent, got %v", err)
	}
}

func TestOneBotReplyHTTPTransportClassifiesGenericUpstreamFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream unavailable"))
	}))
	defer server.Close()

	sender := NewOneBotReplySender(NewOneBotReplyHTTPTransport(server.URL, server.Client()))

	_, err := sender.ReplyText(eventmodel.ReplyHandle{
		Capability: "onebot.reply",
		TargetID:   "group-42",
		Metadata: map[string]any{
			"message_type": "group",
			"group_id":     int64(42),
		},
	}, "pong")
	if err == nil {
		t.Fatal("expected generic upstream failure")
	}

	var httpErr *OneBotReplyHTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected OneBotReplyHTTPError, got %T (%v)", err, err)
	}
	if httpErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %+v", httpErr)
	}
	if httpErr.Status != "502 Bad Gateway" {
		t.Fatalf("expected http status text, got %+v", httpErr)
	}
	if httpErr.Body != "upstream unavailable" {
		t.Fatalf("expected upstream failure body to be preserved, got %+v", httpErr)
	}
	if httpErr.ResponseBodyTruncated() || httpErr.BodyTruncated {
		t.Fatalf("expected non-oversized upstream failure body to remain untruncated, got %+v", httpErr)
	}
	if httpErr.Timeout() {
		t.Fatalf("expected generic upstream failure to remain distinct from timeout, got %+v", httpErr)
	}
	if httpErr.RateLimited() {
		t.Fatalf("expected 502 to remain distinct from rate limit, got %+v", httpErr)
	}
	if httpErr.ResponseReadFailure() || httpErr.NonSuccessResponseReadFailure() || httpErr.ResponseDecodeFailure() || httpErr.OversizedResponseBody() {
		t.Fatalf("expected generic upstream failure to remain distinct from response read/decode/oversized failures, got %+v", httpErr)
	}
	if !httpErr.GenericUpstreamFailure() {
		t.Fatalf("expected 502 to retain generic upstream failure classification, got %+v", httpErr)
	}
	if !httpErr.UpstreamFailure() {
		t.Fatalf("expected 502 to be classified as upstream http failure, got %+v", httpErr)
	}
	if !strings.Contains(err.Error(), "502") || !strings.Contains(err.Error(), "upstream unavailable") {
		t.Fatalf("expected error string to retain status/body detail, got %v", err)
	}
	if strings.Contains(err.Error(), "body truncated") || strings.Contains(err.Error(), "partial body preserved") {
		t.Fatalf("expected untruncated upstream failure to omit truncation hint, got %v", err)
	}
}

func TestOneBotReplyHTTPTransportClassifiesGenericUpstreamFailureWithTruncatedBodyHint(t *testing.T) {
	t.Parallel()

	oversizedBody := strings.Repeat("x", oneBotReplyHTTPResponseBodyLimit+1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(oversizedBody))
	}))
	defer server.Close()

	sender := NewOneBotReplySender(NewOneBotReplyHTTPTransport(server.URL, server.Client()))

	_, err := sender.ReplyText(eventmodel.ReplyHandle{
		Capability: "onebot.reply",
		TargetID:   "group-42",
		Metadata: map[string]any{
			"message_type": "group",
			"group_id":     int64(42),
		},
	}, "pong")
	if err == nil {
		t.Fatal("expected generic upstream failure")
	}

	var httpErr *OneBotReplyHTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected OneBotReplyHTTPError, got %T (%v)", err, err)
	}
	if httpErr.StatusCode != http.StatusBadGateway || httpErr.Status != "502 Bad Gateway" {
		t.Fatalf("expected oversized non-success response metadata to be preserved, got %+v", httpErr)
	}
	if len(httpErr.Body) != oneBotReplyHTTPResponseBodyLimit {
		t.Fatalf("expected truncated non-success body length %d, got %d", oneBotReplyHTTPResponseBodyLimit, len(httpErr.Body))
	}
	if httpErr.Body != oversizedBody[:oneBotReplyHTTPResponseBodyLimit] {
		t.Fatalf("expected truncated non-success body prefix to be preserved, got %+v", httpErr)
	}
	if !httpErr.ResponseBodyTruncated() || !httpErr.BodyTruncated {
		t.Fatalf("expected oversized non-success response body truncation hint, got %+v", httpErr)
	}
	if httpErr.Timeout() || httpErr.RateLimited() || httpErr.ResponseReadFailure() || httpErr.NonSuccessResponseReadFailure() || httpErr.ResponseDecodeFailure() || httpErr.OversizedResponseBody() {
		t.Fatalf("expected truncated non-success body hint to remain distinct from timeout/rate-limit/read/decode/success-body-oversized classifications, got %+v", httpErr)
	}
	if !httpErr.GenericUpstreamFailure() || !httpErr.UpstreamFailure() {
		t.Fatalf("expected truncated non-success body to retain upstream failure classification, got %+v", httpErr)
	}
	if !strings.Contains(err.Error(), "502") {
		t.Fatalf("expected error string to retain non-success status detail, got %v", err)
	}
	if !strings.Contains(err.Error(), "body truncated") || !strings.Contains(err.Error(), "partial body preserved") {
		t.Fatalf("expected truncated non-success error string to expose truncation hint, got %v", err)
	}
}

func TestOneBotReplyHTTPTransportClassifiesNonSuccessResponseReadFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("expected hijacker support")
		}
		conn, buffer, err := hijacker.Hijack()
		if err != nil {
			t.Fatalf("hijack response writer: %v", err)
		}
		defer conn.Close()

		_, _ = buffer.WriteString("HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\nContent-Length: 22\r\nConnection: close\r\n\r\nupstream unavailable")
		_ = buffer.Flush()
	}))
	defer server.Close()

	sender := NewOneBotReplySender(NewOneBotReplyHTTPTransport(server.URL, server.Client()))

	_, err := sender.ReplyText(eventmodel.ReplyHandle{
		Capability: "onebot.reply",
		TargetID:   "group-42",
		Metadata: map[string]any{
			"message_type": "group",
			"group_id":     int64(42),
		},
	}, "pong")
	if err == nil {
		t.Fatal("expected non-success response-read failure")
	}

	var httpErr *OneBotReplyHTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected OneBotReplyHTTPError, got %T (%v)", err, err)
	}
	if httpErr.StatusCode != http.StatusBadGateway || httpErr.Status != "502 Bad Gateway" {
		t.Fatalf("expected non-success response metadata to be preserved, got %+v", httpErr)
	}
	if httpErr.Body != "upstream unavailable" {
		t.Fatalf("expected partial non-success body prefix to be preserved on read failure, got %+v", httpErr)
	}
	if !httpErr.ResponseReadFailure() || !httpErr.NonSuccessResponseReadFailure() || !httpErr.ResponseReadFailed {
		t.Fatalf("expected non-success response-read classification, got %+v", httpErr)
	}
	if httpErr.ResponseDecodeFailure() {
		t.Fatalf("expected non-success response-read failure to remain distinct from decode failure, got %+v", httpErr)
	}
	if httpErr.Timeout() || httpErr.RateLimited() || httpErr.GenericUpstreamFailure() || httpErr.UpstreamFailure() {
		t.Fatalf("expected non-success response-read failure to remain distinct from timeout/rate-limit/generic upstream failure, got %+v", httpErr)
	}
	if !errors.Is(httpErr.Err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected underlying read error to retain unexpected EOF, got %v", httpErr.Err)
	}
	if !strings.Contains(err.Error(), "response read failed") || !strings.Contains(err.Error(), "502 Bad Gateway") {
		t.Fatalf("expected error string to expose non-success read-failure classification, got %v", err)
	}
	if !strings.Contains(err.Error(), "partial body preserved") || !strings.Contains(err.Error(), "upstream unavailable") {
		t.Fatalf("expected error string to expose preserved non-success partial body hint, got %v", err)
	}
}

func TestOneBotReplyHTTPTransportClassifiesResponseReadFailure(t *testing.T) {
	t.Parallel()

	const partialBodyPrefix = `{"message_id":`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("expected hijacker support")
		}
		conn, buffer, err := hijacker.Hijack()
		if err != nil {
			t.Fatalf("hijack response writer: %v", err)
		}
		defer conn.Close()

		_, _ = buffer.WriteString("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 24\r\nConnection: close\r\n\r\n" + partialBodyPrefix)
		_ = buffer.Flush()
	}))
	defer server.Close()

	sender := NewOneBotReplySender(NewOneBotReplyHTTPTransport(server.URL, server.Client()))

	_, err := sender.ReplyText(eventmodel.ReplyHandle{
		Capability: "onebot.reply",
		TargetID:   "group-42",
		Metadata: map[string]any{
			"message_type": "group",
			"group_id":     int64(42),
		},
	}, "pong")
	if err == nil {
		t.Fatal("expected response-read failure")
	}

	var httpErr *OneBotReplyHTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected OneBotReplyHTTPError, got %T (%v)", err, err)
	}
	if httpErr.StatusCode != http.StatusOK || httpErr.Status != "200 OK" {
		t.Fatalf("expected success-status response metadata to be preserved, got %+v", httpErr)
	}
	if httpErr.Body != partialBodyPrefix {
		t.Fatalf("expected success-status read failure to preserve partial body prefix, got %+v", httpErr)
	}
	if !httpErr.ResponseReadFailure() || !httpErr.ResponseReadFailed {
		t.Fatalf("expected response-read classification, got %+v", httpErr)
	}
	if httpErr.NonSuccessResponseReadFailure() {
		t.Fatalf("expected success-status read failure to remain distinct from non-success read failure, got %+v", httpErr)
	}
	if httpErr.ResponseDecodeFailure() || httpErr.OversizedResponseBody() {
		t.Fatalf("expected response-read failure to remain distinct from decode/oversized failure, got %+v", httpErr)
	}
	if httpErr.Timeout() || httpErr.RateLimited() || httpErr.GenericUpstreamFailure() || httpErr.UpstreamFailure() {
		t.Fatalf("expected response-read failure to remain distinct from timeout/rate-limit/upstream failure, got %+v", httpErr)
	}
	if !errors.Is(httpErr.Err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected underlying read error to retain unexpected EOF, got %v", httpErr.Err)
	}
	if !strings.Contains(err.Error(), "response read failed") || !strings.Contains(err.Error(), "200 OK") {
		t.Fatalf("expected error string to expose read-failure classification, got %v", err)
	}
	if !strings.Contains(err.Error(), "partial body preserved") || !strings.Contains(err.Error(), partialBodyPrefix) {
		t.Fatalf("expected error string to expose preserved success-status partial body hint, got %v", err)
	}
}

func TestOneBotReplyHTTPTransportClassifiesOversizedSuccessResponseBody(t *testing.T) {
	t.Parallel()

	const successBodyPrefix = `{"message_id": 4005}`
	oversizedBody := successBodyPrefix + strings.Repeat(" ", oneBotReplyHTTPResponseBodyLimit-len(successBodyPrefix)+1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(oversizedBody))
	}))
	defer server.Close()

	sender := NewOneBotReplySender(NewOneBotReplyHTTPTransport(server.URL, server.Client()))

	_, err := sender.ReplyText(eventmodel.ReplyHandle{
		Capability: "onebot.reply",
		TargetID:   "group-42",
		Metadata: map[string]any{
			"message_type": "group",
			"group_id":     int64(42),
		},
	}, "pong")
	if err == nil {
		t.Fatal("expected oversized success-status response-body failure")
	}

	var httpErr *OneBotReplyHTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected OneBotReplyHTTPError, got %T (%v)", err, err)
	}
	if httpErr.StatusCode != http.StatusOK || httpErr.Status != "200 OK" {
		t.Fatalf("expected success-status response metadata to be preserved, got %+v", httpErr)
	}
	if len(httpErr.Body) != oneBotReplyHTTPResponseBodyLimit {
		t.Fatalf("expected oversized response body to be truncated to %d bytes, got %d", oneBotReplyHTTPResponseBodyLimit, len(httpErr.Body))
	}
	if httpErr.Body != oversizedBody[:oneBotReplyHTTPResponseBodyLimit] {
		t.Fatalf("expected oversized response body to retain the 4KiB truncated prefix, got %+v", httpErr)
	}
	if !httpErr.OversizedResponseBody() || !httpErr.ResponseBodyTooLarge {
		t.Fatalf("expected oversized response-body classification, got %+v", httpErr)
	}
	if httpErr.ResponseReadFailure() || httpErr.NonSuccessResponseReadFailure() || httpErr.ResponseDecodeFailure() {
		t.Fatalf("expected oversized success-status body to remain distinct from read/decode failures, got %+v", httpErr)
	}
	if httpErr.Timeout() || httpErr.RateLimited() || httpErr.GenericUpstreamFailure() || httpErr.UpstreamFailure() {
		t.Fatalf("expected oversized success-status body to remain distinct from timeout/rate-limit/upstream failure, got %+v", httpErr)
	}
	if !strings.Contains(err.Error(), "exceeded 4096 bytes limit") || !strings.Contains(err.Error(), "200 OK") {
		t.Fatalf("expected error string to expose oversized-body classification, got %v", err)
	}
}

func TestOneBotReplyHTTPTransportResponseReadFailurePreservesCompatibilityBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("non-success read failure still preserves prefix and classification", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("expected hijacker support")
			}
			conn, buffer, err := hijacker.Hijack()
			if err != nil {
				t.Fatalf("hijack response writer: %v", err)
			}
			defer conn.Close()

			_, _ = buffer.WriteString("HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\nContent-Length: 22\r\nConnection: close\r\n\r\nupstream unavailable")
			_ = buffer.Flush()
		}))
		defer server.Close()

		sender := NewOneBotReplySender(NewOneBotReplyHTTPTransport(server.URL, server.Client()))

		_, err := sender.ReplyText(eventmodel.ReplyHandle{
			Capability: "onebot.reply",
			TargetID:   "group-42",
			Metadata: map[string]any{
				"message_type": "group",
				"group_id":     int64(42),
			},
		}, "pong")
		if err == nil {
			t.Fatal("expected non-success response-read failure")
		}

		var httpErr *OneBotReplyHTTPError
		if !errors.As(err, &httpErr) {
			t.Fatalf("expected OneBotReplyHTTPError, got %T (%v)", err, err)
		}
		if httpErr.Body != "upstream unavailable" {
			t.Fatalf("expected non-success response-read failure body to remain unchanged, got %+v", httpErr)
		}
		if !httpErr.ResponseReadFailure() || !httpErr.NonSuccessResponseReadFailure() {
			t.Fatalf("expected non-success response-read classification to remain unchanged, got %+v", httpErr)
		}
		if httpErr.ResponseDecodeFailure() || httpErr.OversizedResponseBody() || httpErr.RateLimited() || httpErr.UpstreamFailure() {
			t.Fatalf("expected non-success response-read failure boundaries to remain unchanged, got %+v", httpErr)
		}
	})

	t.Run("decode failure remains distinct from read failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"message_id":"oops"}`))
		}))
		defer server.Close()

		sender := NewOneBotReplySender(NewOneBotReplyHTTPTransport(server.URL, server.Client()))

		_, err := sender.ReplyText(eventmodel.ReplyHandle{
			Capability: "onebot.reply",
			TargetID:   "group-42",
			Metadata: map[string]any{
				"message_type": "group",
				"group_id":     int64(42),
			},
		}, "pong")
		if err == nil {
			t.Fatal("expected response-decode failure")
		}

		var httpErr *OneBotReplyHTTPError
		if !errors.As(err, &httpErr) {
			t.Fatalf("expected OneBotReplyHTTPError, got %T (%v)", err, err)
		}
		if !httpErr.ResponseDecodeFailure() || httpErr.ResponseReadFailure() || httpErr.NonSuccessResponseReadFailure() {
			t.Fatalf("expected decode failure classification to remain unchanged, got %+v", httpErr)
		}
	})
}

func TestOneBotReplyHTTPTransportClassifiesResponseDecodeFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message_id":"oops"}`))
	}))
	defer server.Close()

	sender := NewOneBotReplySender(NewOneBotReplyHTTPTransport(server.URL, server.Client()))

	_, err := sender.ReplyText(eventmodel.ReplyHandle{
		Capability: "onebot.reply",
		TargetID:   "group-42",
		Metadata: map[string]any{
			"message_type": "group",
			"group_id":     int64(42),
		},
	}, "pong")
	if err == nil {
		t.Fatal("expected response-decode failure")
	}

	var httpErr *OneBotReplyHTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected OneBotReplyHTTPError, got %T (%v)", err, err)
	}
	if httpErr.StatusCode != http.StatusOK || httpErr.Status != "200 OK" {
		t.Fatalf("expected success-status response metadata to be preserved, got %+v", httpErr)
	}
	if httpErr.Body != `{"message_id":"oops"}` {
		t.Fatalf("expected decode-failure body to be preserved, got %+v", httpErr)
	}
	if !httpErr.ResponseDecodeFailure() || !httpErr.ResponseDecodeFailed {
		t.Fatalf("expected response-decode classification, got %+v", httpErr)
	}
	if httpErr.ResponseReadFailure() || httpErr.OversizedResponseBody() {
		t.Fatalf("expected decode failure to remain distinct from read/oversized failure, got %+v", httpErr)
	}
	if httpErr.Timeout() || httpErr.RateLimited() || httpErr.GenericUpstreamFailure() || httpErr.UpstreamFailure() {
		t.Fatalf("expected decode failure to remain distinct from timeout/rate-limit/upstream failure, got %+v", httpErr)
	}
	if !strings.Contains(err.Error(), "response decode failed") || !strings.Contains(err.Error(), "200 OK") {
		t.Fatalf("expected error string to expose decode-failure classification, got %v", err)
	}
}
