package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/crab4/gmail-cleaner/models"
)

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type OllamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func askOllamaOld(prompt string) (string, error) {
	url := "http://localhost:11434/api/generate"
	reqBody := OllamaRequest{
		Model:  "qwen2.5:0.5b",
		Prompt: prompt,
		Stream: false,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("Oshibka v marshalinge zaprosa: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("oshibka sozdaniya HTTP zaprosa: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Oshibka send HTTP zaprosa:%w", err)
	}
	//defer - аналог using, он утверждает что Close выполнится. Подсказка себе из прошлого
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Oshibka chteniya otveta: %w", err)
	}

	var ollamaResp OllamaResponse
	err = json.Unmarshal(body, &ollamaResp)
	if err != nil {
		return "", fmt.Errorf("error v parsinge otveta %w, telo %s", err, string(body))
	}
	return ollamaResp.Response, nil
}

func askOllama(ctx context.Context, cfg models.Config, prompt string) (string, error) {
	url := cfg.OllamaUrl + "/api/generate"

	reqBody := OllamaRequest{
		Model:  cfg.OllamaModel,
		Prompt: prompt,
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("Ошибка при маршалинге запроса %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("ошибка при создании запроса%v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка при хттп запросе %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ошибка чтения ответа %v", err)
	}

	var ollamaResp OllamaResponse
	err = json.Unmarshal(body, &ollamaResp)
	if err != nil {
		return "", fmt.Errorf("ошибка при парсинге ответа %w тело:%s", err, string(body))
	}
	return ollamaResp.Response, nil
}
