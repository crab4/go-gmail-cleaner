package main

import (
	"fmt"
	"log"
)

func main() {
	prompt := "Скажи 'Привет мир!' и больше ничего."
	answer, err := askOllama(prompt)
	if err != nil {
		log.Fatalf("Ne ydalos obratitsya k ollama %v", err)
	}
	fmt.Println("response:", answer)
}
