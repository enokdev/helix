package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// mockContext implements Context with a configurable JSON error.
type mockContext struct {
	jsonErr    error
	statusCode int
	method     string
}

func (m *mockContext) Method() string                { return m.method }
func (m *mockContext) Path() string                  { return "/" }
func (m *mockContext) OriginalURL() string           { return "" }
func (m *mockContext) Param(_ string) string         { return "" }
func (m *mockContext) Query(_ string) string         { return "" }
func (m *mockContext) Header(_ string) string        { return "" }
func (m *mockContext) IP() string                    { return "" }
func (m *mockContext) Body() []byte                  { return nil }
func (m *mockContext) Status(code int)               { m.statusCode = code }
func (m *mockContext) SetHeader(_, _ string)         {}
func (m *mockContext) Send(_ []byte) error           { return nil }
func (m *mockContext) JSON(_ any) error              { return m.jsonErr }
func (m *mockContext) Locals(_ string, _ ...any) any { return nil }

func TestWriteSuccessResponse_JSONError_IsLoggedWithWebNamespace(t *testing.T) {
	orig := slog.Default()
	t.Cleanup(func() { slog.SetDefault(orig) })

	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))

	jsonErr := errors.New("json: unsupported type: chan int")
	ctx := &mockContext{method: "GET", jsonErr: jsonErr}

	err := writeSuccessResponse(ctx, "GET", make(chan int))
	if err == nil {
		t.Fatal("writeSuccessResponse() should propagate JSON error")
	}
	if !errors.Is(err, jsonErr) {
		t.Errorf("err = %v, want jsonErr", err)
	}

	output := buf.String()
	if !strings.Contains(output, "web") {
		t.Errorf("expected namespace 'web' in log output, got: %s", output)
	}

	var entry map[string]any
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if ns, ok := entry["namespace"]; ok && ns == "web" {
			return
		}
	}
	t.Errorf("no log line with namespace='web' found in output: %s", output)
}

func TestWriteSuccessResponse_JSONError_PropagatesError(t *testing.T) {
	t.Parallel()

	jsonErr := errors.New("json: unsupported type")
	ctx := &mockContext{method: "POST", jsonErr: jsonErr}

	err := writeSuccessResponse(ctx, "POST", struct{ Ch chan int }{})
	if err == nil {
		t.Fatal("writeSuccessResponse() should return error when JSON fails")
	}
}

func TestWriteSuccessResponse_Success_ReturnsNil(t *testing.T) {
	t.Parallel()

	ctx := &mockContext{method: "GET"}
	err := writeSuccessResponse(ctx, "GET", map[string]string{"key": "val"})
	if err != nil {
		t.Errorf("writeSuccessResponse() error = %v, want nil", err)
	}
}
