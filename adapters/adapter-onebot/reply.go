package adapteronebot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	eventmodel "github.com/ohmyopencode/bot-platform/packages/event-model"
)

type SenderTransport func(OneBotSendRequest) (OneBotSendResponse, error)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

const oneBotReplyHTTPResponseBodyLimit = 4 << 10

type OneBotReplySender struct {
	transport SenderTransport
}

type OneBotSendRequest struct {
	Action string         `json:"action"`
	Params map[string]any `json:"params"`
}

type OneBotSendResponse struct {
	MessageID int64 `json:"message_id"`
}

type ReplyResult struct {
	MessageID string `json:"message_id"`
}

type OneBotReplyHTTPError struct {
	StatusCode           int
	Status               string
	Body                 string
	RetryAfter           string
	TimedOut             bool
	ResponseReadFailed   bool
	ResponseDecodeFailed bool
	ResponseBodyTooLarge bool
	BodyTruncated        bool
	Err                  error
}

func (e *OneBotReplyHTTPError) Error() string {
	if e == nil {
		return "onebot reply http request failed"
	}
	retryAfterHint := ""
	if e.StatusCode == http.StatusTooManyRequests && e.RetryAfter != "" {
		retryAfterHint = fmt.Sprintf(" (retry-after: %s)", e.RetryAfter)
	}
	if e.TimedOut && e.Err != nil {
		return fmt.Sprintf("onebot reply http request timed out: %v", e.Err)
	}
	if e.ResponseReadFailed && e.Err != nil {
		partialBodyHint := ""
		if e.Body != "" {
			partialBodyHint = fmt.Sprintf(" (partial body preserved: %s)", e.Body)
		}
		if e.StatusCode != 0 {
			return fmt.Sprintf("onebot reply http response read failed after %d %s: %v%s", e.StatusCode, e.Status, e.Err, partialBodyHint)
		}
		return fmt.Sprintf("onebot reply http response read failed: %v%s", e.Err, partialBodyHint)
	}
	if e.ResponseBodyTooLarge {
		if e.StatusCode != 0 {
			return fmt.Sprintf("onebot reply http response body exceeded %d bytes limit after %d %s", oneBotReplyHTTPResponseBodyLimit, e.StatusCode, e.Status)
		}
		return fmt.Sprintf("onebot reply http response body exceeded %d bytes limit", oneBotReplyHTTPResponseBodyLimit)
	}
	if e.ResponseDecodeFailed && e.Err != nil {
		if e.StatusCode != 0 {
			return fmt.Sprintf("onebot reply http response decode failed after %d %s: %v", e.StatusCode, e.Status, e.Err)
		}
		return fmt.Sprintf("onebot reply http response decode failed: %v", e.Err)
	}
	if e.StatusCode != 0 && e.Body != "" {
		if e.BodyTruncated {
			return fmt.Sprintf("onebot reply http status %d %s: %s (body truncated; partial body preserved)%s", e.StatusCode, e.Status, e.Body, retryAfterHint)
		}
		return fmt.Sprintf("onebot reply http status %d %s: %s%s", e.StatusCode, e.Status, e.Body, retryAfterHint)
	}
	if e.StatusCode != 0 {
		if e.BodyTruncated {
			return fmt.Sprintf("onebot reply http status %d %s (body truncated; partial body preserved)%s", e.StatusCode, e.Status, retryAfterHint)
		}
		return fmt.Sprintf("onebot reply http status %d %s%s", e.StatusCode, e.Status, retryAfterHint)
	}
	if e.Err != nil {
		return fmt.Sprintf("onebot reply http request failed: %v", e.Err)
	}
	return "onebot reply http request failed"
}

func (e *OneBotReplyHTTPError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *OneBotReplyHTTPError) Timeout() bool {
	return e != nil && e.TimedOut
}

func (e *OneBotReplyHTTPError) RateLimited() bool {
	return e != nil && e.StatusCode == http.StatusTooManyRequests
}

func (e *OneBotReplyHTTPError) GenericUpstreamFailure() bool {
	return e != nil && !e.TimedOut && !e.ResponseReadFailed && !e.ResponseDecodeFailed && !e.ResponseBodyTooLarge && e.StatusCode >= http.StatusBadRequest && e.StatusCode != http.StatusTooManyRequests
}

func (e *OneBotReplyHTTPError) ResponseReadFailure() bool {
	return e != nil && e.ResponseReadFailed
}

func (e *OneBotReplyHTTPError) NonSuccessResponseReadFailure() bool {
	return e != nil && e.ResponseReadFailed && (e.StatusCode < http.StatusOK || e.StatusCode >= http.StatusMultipleChoices)
}

func (e *OneBotReplyHTTPError) OversizedResponseBody() bool {
	return e != nil && e.ResponseBodyTooLarge
}

func (e *OneBotReplyHTTPError) ResponseBodyTruncated() bool {
	return e != nil && e.BodyTruncated
}

func (e *OneBotReplyHTTPError) ResponseDecodeFailure() bool {
	return e != nil && e.ResponseDecodeFailed
}

func (e *OneBotReplyHTTPError) UpstreamFailure() bool {
	return e != nil && (e.RateLimited() || e.GenericUpstreamFailure())
}

func NewOneBotReplySender(transport SenderTransport) *OneBotReplySender {
	return &OneBotReplySender{transport: transport}
}

func NewOneBotReplyHTTPTransport(endpoint string, client HTTPDoer) SenderTransport {
	return func(request OneBotSendRequest) (OneBotSendResponse, error) {
		if strings.TrimSpace(endpoint) == "" {
			return OneBotSendResponse{}, errors.New("onebot reply http endpoint is required")
		}
		if client == nil {
			return OneBotSendResponse{}, errors.New("onebot reply http client is required")
		}

		payload, err := json.Marshal(request)
		if err != nil {
			return OneBotSendResponse{}, fmt.Errorf("marshal onebot reply request: %w", err)
		}

		httpRequest, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return OneBotSendResponse{}, fmt.Errorf("build onebot reply request: %w", err)
		}
		httpRequest.Header.Set("Content-Type", "application/json")

		response, err := client.Do(httpRequest)
		if err != nil {
			return OneBotSendResponse{}, &OneBotReplyHTTPError{Err: err, TimedOut: isTimeoutError(err)}
		}
		defer response.Body.Close()

		body, err := io.ReadAll(io.LimitReader(response.Body, oneBotReplyHTTPResponseBodyLimit+1))
		if err != nil {
			trimmedBody := strings.TrimSpace(string(body))
			return OneBotSendResponse{}, &OneBotReplyHTTPError{
				StatusCode:         response.StatusCode,
				Status:             response.Status,
				Body:               trimmedBody,
				ResponseReadFailed: true,
				Err:                err,
			}
		}
		bodyExceededLimit := len(body) > oneBotReplyHTTPResponseBodyLimit
		if bodyExceededLimit {
			body = body[:oneBotReplyHTTPResponseBodyLimit]
		}
		trimmedBody := strings.TrimSpace(string(body))
		retryAfter := ""
		if response.StatusCode == http.StatusTooManyRequests {
			retryAfter = strings.TrimSpace(response.Header.Get("Retry-After"))
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			return OneBotSendResponse{}, &OneBotReplyHTTPError{
				StatusCode:    response.StatusCode,
				Status:        response.Status,
				Body:          trimmedBody,
				RetryAfter:    retryAfter,
				BodyTruncated: bodyExceededLimit,
			}
		}
		if bodyExceededLimit {
			return OneBotSendResponse{}, &OneBotReplyHTTPError{
				StatusCode:           response.StatusCode,
				Status:               response.Status,
				Body:                 string(body),
				ResponseBodyTooLarge: true,
			}
		}

		var decoded OneBotSendResponse
		if err := json.Unmarshal(body, &decoded); err != nil {
			return OneBotSendResponse{}, &OneBotReplyHTTPError{
				StatusCode:           response.StatusCode,
				Status:               response.Status,
				Body:                 trimmedBody,
				ResponseDecodeFailed: true,
				Err:                  err,
			}
		}
		return decoded, nil
	}
}

func ReplyHandleFromMessageEvent(payload MessageEventPayload) *eventmodel.ReplyHandle {
	handle := &eventmodel.ReplyHandle{
		Capability: "onebot.reply",
		TargetID:   channelID(payload),
		MessageID:  fmt.Sprintf("msg-%d", payload.MessageID),
		Metadata: map[string]any{
			"message_type": payload.MessageType,
			"user_id":      payload.UserID,
		},
	}
	if payload.MessageType == "group" && payload.GroupID != 0 {
		handle.Metadata["group_id"] = payload.GroupID
	}
	return handle
}

func (s *OneBotReplySender) ReplyText(handle eventmodel.ReplyHandle, text string) (ReplyResult, error) {
	request, err := s.requestFor(handle, text)
	if err != nil {
		return ReplyResult{}, err
	}
	return s.send(request)
}

func (s *OneBotReplySender) ReplyImage(handle eventmodel.ReplyHandle, imageURL string) (ReplyResult, error) {
	request, err := s.requestFor(handle, fmt.Sprintf("[CQ:image,file=%s]", imageURL))
	if err != nil {
		return ReplyResult{}, err
	}
	return s.send(request)
}

func (s *OneBotReplySender) ReplyFile(handle eventmodel.ReplyHandle, fileURL string) (ReplyResult, error) {
	request, err := s.requestFor(handle, fmt.Sprintf("[CQ:file,file=%s]", fileURL))
	if err != nil {
		return ReplyResult{}, err
	}
	return s.send(request)
}

func (s *OneBotReplySender) send(request OneBotSendRequest) (ReplyResult, error) {
	if s.transport == nil {
		return ReplyResult{}, errors.New("sender transport is required")
	}
	response, err := s.transport(request)
	if err != nil {
		return ReplyResult{}, err
	}
	return ReplyResult{MessageID: fmt.Sprintf("msg-%d", response.MessageID)}, nil
}

func (s *OneBotReplySender) requestFor(handle eventmodel.ReplyHandle, message string) (OneBotSendRequest, error) {
	messageType, _ := handle.Metadata["message_type"].(string)
	params := map[string]any{"message": message}

	switch messageType {
	case "group":
		groupID, err := int64Metadata(handle.Metadata, "group_id")
		if err != nil {
			return OneBotSendRequest{}, err
		}
		params["group_id"] = groupID
	case "private":
		userID, err := int64Metadata(handle.Metadata, "user_id")
		if err != nil {
			return OneBotSendRequest{}, err
		}
		params["user_id"] = userID
	default:
		return OneBotSendRequest{}, fmt.Errorf("unsupported reply message_type %q", messageType)
	}

	return OneBotSendRequest{Action: "send_msg", Params: params}, nil
}

func int64Metadata(metadata map[string]any, key string) (int64, error) {
	value, exists := metadata[key]
	if !exists {
		return 0, fmt.Errorf("reply metadata missing %s", key)
	}

	switch typed := value.(type) {
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case float64:
		return int64(typed), nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse %s: %w", key, err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported metadata type for %s", key)
	}
}

func isTimeoutError(err error) bool {
	var timeoutErr interface{ Timeout() bool }
	return errors.As(err, &timeoutErr) && timeoutErr.Timeout()
}
