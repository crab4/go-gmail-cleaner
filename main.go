package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/crab4/gmail-cleaner/models"
)

// TODO: дописать config.go, куда скинуть инфу о конфиге, о кол-ве писем, о том, к какой модели мы подключаемся
// TODO: дописать автоматический подъём ollama, если юзер её не поднял, делов на 40-50 строк кода, но пока лень
// Пока что создадим просто ConfigStruct

func main() {
	config := models.LoadConfig()

	srv, err := getGmailService()
	if err != nil {
		log.Fatalf("Не удалось подключиться к Gmail %v", err)
	}

	fmt.Println("Гмейл сервис готов")
	ids, err := listMessageIds(srv, config.GmailMaxResults)
	if err != nil {
		log.Fatalf("Ошибка получения списка %v", err)
	}
	fmt.Printf("получено %d ID писем\n", len(ids))

	emails, err := fetchEmails(srv, ids, config.GmailWorkers)
	if err != nil {
		log.Printf("Загрузка писем завершилась ошибкой %v", err)
	}

	fmt.Printf("\nЗагруженные письма:")
	for i, message := range emails {
		fmt.Printf("%d. id %s: subject:%s, snippet:%s\n", i, message.ID, message.Subject, message.Snippet)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	classified := classifyEmails(ctx, config, emails)

	fmt.Println("\nРезультаты классификации:")
	for _, c := range classified {
		status := "KEEP"
		if c.IsSpam {
			status = "SPAM"
		}
		fmt.Printf("[%s] %s => %s\n", c.Email.ID, c.Email.Subject, status)
	}
}
