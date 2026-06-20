package models

type Config struct {
	OllamaUrl       string `json:"ollama_url"`
	OllamaModel     string `json:"ollama_model"`
	GmailMaxResults int64  `json:"gmail_max_results"`
	OllamaWorkers   int    `json:"ollama_workers"`
	GmailWorkers    int    `json:"gmail_workers"`
}

func LoadConfig() Config {
	return Config{
		OllamaUrl:       "http://localhost:11434",
		OllamaModel:     "qwen2.5:0.5b",
		GmailMaxResults: 100,
		OllamaWorkers:   3,
		GmailWorkers:    5,
	}
}
