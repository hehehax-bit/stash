package api

type AIChatSendInput struct {
	SessionID         *string `json:"session_id"`
	Message           string  `json:"message"`
	Image             *string `json:"image"`
	UseLibraryContext *bool   `json:"use_library_context"`
}

type AIScheduledTasksConfig struct {
	EmbeddingRefreshHours int `json:"embedding_refresh_hours"`
	AudioAnalysisHours    int `json:"audio_analysis_hours"`
}

type AIScheduledTasksConfigInput struct {
	EmbeddingRefreshHours *int `json:"embedding_refresh_hours"`
	AudioAnalysisHours    *int `json:"audio_analysis_hours"`
}
