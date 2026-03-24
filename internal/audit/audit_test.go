package audit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockObserver struct {
	events []Event
	mu     sync.Mutex
}

func (m *MockObserver) OnEvent(event Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
}

func TestAuditor_Notify(t *testing.T) {
	auditor := NewAuditor()
	mock := &MockObserver{}
	auditor.Register(mock)

	event := Event{
		Timestamp: time.Now().Unix(),
		Action:    ActionShorten,
		UserID:    "user1",
		URL:       "http://example.com",
	}

	auditor.Notify(event)

	// Wait for async notification
	time.Sleep(100 * time.Millisecond)

	mock.mu.Lock()
	defer mock.mu.Unlock()
	require.Equal(t, 1, len(mock.events))
	assert.Equal(t, event, mock.events[0])
}

func TestFileObserver_OnEvent(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "audit_log_*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	observer := NewFileObserver(tmpFile.Name())
	event := Event{
		Timestamp: 12345678,
		Action:    ActionShorten,
		UserID:    "user123",
		URL:       "http://test.com",
	}

	observer.OnEvent(event)

	data, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)

	var savedEvent Event
	err = json.Unmarshal(data, &savedEvent)
	require.NoError(t, err)
	assert.Equal(t, event, savedEvent)
}

func TestURLObserver_OnEvent(t *testing.T) {
	var capturedEvent Event
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&capturedEvent)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	observer := NewURLObserver(server.URL)
	event := Event{
		Timestamp: 12345678,
		Action:    ActionFollow,
		UserID:    "user456",
		URL:       "http://test-follow.com",
	}

	observer.OnEvent(event)

	assert.Equal(t, event, capturedEvent)
}

func TestAuditor_Audit(t *testing.T) {
	auditor := NewAuditor()
	mock := &MockObserver{}
	auditor.Register(mock)

	auditor.Audit(ActionShorten, "user1", "http://example.com")

	// Wait for async notification
	time.Sleep(100 * time.Millisecond)

	mock.mu.Lock()
	defer mock.mu.Unlock()
	require.Equal(t, 1, len(mock.events))
	assert.Equal(t, ActionShorten, mock.events[0].Action)
	assert.Equal(t, "user1", mock.events[0].UserID)
	assert.Equal(t, "http://example.com", mock.events[0].URL)
	assert.NotZero(t, mock.events[0].Timestamp)
}
