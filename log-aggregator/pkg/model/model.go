package model

type LogEntry struct {
	Source    string `json:"source"`
	Timestamp int64  `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}
