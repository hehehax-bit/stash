package config

// AIScheduledTasksConfig configures periodic AI maintenance tasks. Interval
// values are in hours; 0 disables a task.
type AIScheduledTasksConfig struct {
	EmbeddingRefreshHours int `json:"embedding_refresh_hours"`
	AudioAnalysisHours    int `json:"audio_analysis_hours"`
}
