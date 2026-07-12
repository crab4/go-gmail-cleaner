package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
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

	// Выделяем спамные письма
	spamEmails := make([]models.ClassifiedEmail, 0)
	for _, c := range classified {
		if c.IsSpam {
			spamEmails = append(spamEmails, c)
		}
	}

	if len(spamEmails) == 0 {
		fmt.Println("Спам не найден. Программа завершена.")
		return
	}

	fmt.Println("\nОбнаружены спамные письма (можно удалить):")
	for i, c := range spamEmails {
		fmt.Printf("%d. [ID: %s] %s\n", i+1, c.Email.ID, c.Email.Subject)
	}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("Введите номера писем для удаления через запятую (например: 1,3,5) или 'all' (все спам), 'none' (ничего): ")
	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())
	if err := scanner.Err(); err != nil {
		log.Fatalf("Ошибка ввода: %v", err)
	}

	var selectedIDs []string

	switch strings.ToLower(input) {
	case "all":
		for _, c := range spamEmails {
			selectedIDs = append(selectedIDs, c.Email.ID)
		}
	case "none", "":
		fmt.Println("Удаление отменено.")
		return
	default:
		parts := strings.Split(input, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			n, err := strconv.Atoi(p)
			if err != nil || n < 1 || n > len(spamEmails) {
				fmt.Printf("Некорректный номер '%s'. Пропускаем.\n", p)
				continue
			}
			selectedIDs = append(selectedIDs, spamEmails[n-1].Email.ID)
		}
	}

	if len(selectedIDs) == 0 {
		fmt.Println("Не выбрано ни одного письма. Удаление отменено.")
		return
	}

	fmt.Print("Точно удалить выбранные письма? (y/N): ")
	scanner.Scan()
	confirm := strings.TrimSpace(scanner.Text())
	if strings.ToLower(confirm) != "y" {
		fmt.Println("Удаление отменено.")
		return
	}

	fmt.Println("Начинаю удаление...")
	for _, id := range selectedIDs {
		if err := trashEmail(srv, id); err != nil {
			log.Printf("Ошибка удаления письма %s: %v", id, err)
			continue
		}
		fmt.Printf("Удалено письмо %s\n", id)
	}
	fmt.Println("Готово.")
}
