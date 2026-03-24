package audit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"
)

type Action string

const (
	ActionShorten Action = "shorten"
	ActionFollow  Action = "follow"
)

type Event struct {
	Timestamp int64  `json:"ts"`
	Action    Action `json:"action"`
	UserID    string `json:"user_id,omitempty"`
	URL       string `json:"url"`
}

type Observer interface {
	OnEvent(event Event)
}

type Auditor struct {
	observers []Observer
	mu        sync.RWMutex
}

func NewAuditor() *Auditor {
	return &Auditor{
		observers: make([]Observer, 0),
	}
}

func (a *Auditor) Register(o Observer) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.observers = append(a.observers, o)
}

func (a *Auditor) Notify(event Event) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, o := range a.observers {
		go o.OnEvent(event)
	}
}

type FileObserver struct {
	filePath string
	mu       sync.Mutex
}

func NewFileObserver(path string) *FileObserver {
	return &FileObserver{filePath: path}
}

func (f *FileObserver) OnEvent(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	file, err := os.OpenFile(f.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	if _, err := file.Write(append(data, '\n')); err != nil {
		return
	}
}

type URLObserver struct {
	url string
}

func NewURLObserver(url string) *URLObserver {
	return &URLObserver{url: url}
}

func (u *URLObserver) OnEvent(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	resp, err := http.Post(u.url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return
	}
	resp.Body.Close()
}

func (a *Auditor) Audit(action Action, userID, url string) {
	event := Event{
		Timestamp: time.Now().Unix(),
		Action:    action,
		UserID:    userID,
		URL:       url,
	}
	a.Notify(event)
}
