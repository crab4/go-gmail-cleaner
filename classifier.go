package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/crab4/gmail-cleaner/models"
)

func classifyEmails(ctx context.Context, cfg models.Config, emails []models.Email) []models.ClassifiedEmail {
	jobs := make(chan models.Email, len(emails))
	results := make(chan models.ClassifiedEmail, len(emails))

	var syncWaiter sync.WaitGroup

	for i := 0; i < cfg.OllamaWorkers; i++ {
		syncWaiter.Add(1)
		go func() {
			defer syncWaiter.Done()
			for email := range jobs {
				isSpam, err := classifyOneMail(ctx, cfg, email)
				if err != nil {
					log.Printf("Ошибка классификации %s %v", email.ID, err)
					isSpam = false
				}
				select {
				case results <- models.ClassifiedEmail{Email: email, IsSpam: isSpam}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, e := range emails {
			select {
			case jobs <- e:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		syncWaiter.Wait()
		close(results)
	}()

	var classified []models.ClassifiedEmail
	for res := range results {
		classified = append(classified, res)
	}
	return classified
}

func classifyOneMail(ctx context.Context, cfg models.Config, email models.Email) (bool, error) {
	prompt := buildPrompt(email)
	answer, err := askOllama(ctx, cfg, prompt)
	if err != nil {
		return false, err
	}

	clean := strings.TrimSpace(strings.ToUpper(answer))
	return clean == "SPAM", nil
}

func buildPrompt(email models.Email) string {
	return fmt.Sprintf(
		`Ты — спам-фильтр. Ответь строго одним словом: SPAM или KEEP.
Письмо: тема "%s", текст "%s".
Это спам или бесполезное уведомление (чеки, билеты, квитанции, реклама), которое можно удалить без последствий? Ответь одним словом.`,
		email.Subject, email.Snippet,
	)
}
