package main

import (
	"fmt"
	"log"
)

func main() {
	srv, err := getGmailService()
	if err != nil {
		log.Fatalf("Не удалось подключиться к Gmail %v", err)
	}

	fmt.Println("Гмейл сервис готов")
	ids, err := listMessageIds(srv, 10)
	if err != nil {
		log.Fatalf("Ошибка получения списка %v", err)
	}
	fmt.Printf("получено %d ID писем\n", len(ids))
}
