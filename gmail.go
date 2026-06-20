package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/crab4/gmail-cleaner/models"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// Собираем здесь сервис и возвращаем его. При первом запуске открывается браузер и сохраняет токен в token.json
// С этого файла уже кукуха начала ехать, поэтому ошибки тоже буду нормально описывать
// Чуть не ёбнулся, пока с ллм разобрался
func getGmailService() (*gmail.Service, error) {
	ctx := context.Background()

	bytes, err := os.ReadFile("credentials.json")
	if err != nil {
		return nil, fmt.Errorf("Не найден credentials.json, %w. Положи его в корень проекта", err)
	}

	//ebychii oauth2
	config, err := google.ConfigFromJSON(bytes, gmail.MailGoogleComScope)
	if err != nil {
		return nil, fmt.Errorf("Парсинг credentials не вышел %w", err)
	}

	client := getClient(ctx, config)
	if client == nil {
		return nil, fmt.Errorf("Не собрался хттп клиент")
	}

	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("Не поднялся сервис %w", err)
	}
	return srv, nil

}

// Берём token.json или запускаем Oauth
func getClient(ctx context.Context, config *oauth2.Config) *http.Client {
	tok, err := tokenFromFile("token.json")
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken("token.json", tok)
	}

	return config.Client(ctx, tok)
}

// ладно, раз уж тут выскочит в браузере, напишем  приилчно
// Переписываем по полный, покрою комментами, та ккак мне тут было неккомфортно
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	//make string делает канал для передачи
	codeChan := make(chan string)
	//Канал о том, что сервер получил код
	serverDone := make(chan struct{})

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(responseWritter http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(responseWritter, "не найден код в юрл", http.StatusBadRequest)
			return
		}
		responseWritter.Write([]byte("Авторизация прошла успешно. Можно закрывать"))
		//посылаем код в канала
		codeChan <- code
		close(serverDone)
	})

	srv := &http.Server{Addr: ":8080", Handler: mux}
	//Моя первая горутина, оууу еее
	//Поднимаем сервак и ждём-с
	go func() {
		fmt.Println("Ожидаю авторизацию на http://localhost:8080...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка запуска хттп сервера %v", err)
		}
	}()

	config.RedirectURL = "http://localhost:8080"
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Нажми на ссылку:\n%v\n", authURL)

	//Опять непривычный код, нужно постараться тебя запомнить
	var code string
	select {
	case code = <-codeChan:
	case <-interruptSignal():
		log.Fatalln("Прервано")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	tok, err := config.Exchange(context.Background(), code)
	if err != nil {
		log.Fatalf("ошибка обмена окда на токен %v", err)
	}
	return tok

}

func interruptSignal() <-chan struct{} {
	c := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		<-sig
		close(c)
	}()
	return c
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(file string, token *oauth2.Token) {
	f, err := os.Create(file)
	if err != nil {
		log.Fatalf("Не могу создать файл с токеном: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func listMessageIds(srv *gmail.Service, maxResult int64) ([]string, error) {
	call := srv.Users.Messages.List("me").MaxResults(maxResult)
	resp, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("Ошибка получения сообщений %w", err)
	}

	ids := make([]string, 0, len(resp.Messages))
	for _, m := range resp.Messages {
		ids = append(ids, m.Id)
	}

	return ids, nil
}

func fetchEmails(srv *gmail.Service, ids []string, numWorkers int) ([]models.Email, error) {
	//Канал контекст с таймаутом, чтобы в случае зависания грохалась
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	//Канал для передчи ID горутинкам. Не уверен в необходимости размерности здесь
	idChan := make(chan string, len(ids))
	resultChan := make(chan models.Email, len(ids))

	var syncWaiter sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		syncWaiter.Add(1)
		go func() {
			defer syncWaiter.Done()
			for id := range idChan {
				email, err := getEmailById(srv, id)
				if err != nil {
					log.Printf("Ошибка при загрузке письма %s, %v", id, err)
					continue
				}
				select {
				case resultChan <- email:
				case <-ctx.Done():
					//Если контекст отмёнен, то закрываемся(подсказка себе любимому)
					return
				}
			}
		}()
	}

	go func() {
		defer close(idChan)
		for _, id := range ids {
			select {
			case idChan <- id:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		syncWaiter.Wait()
		close(resultChan)
	}()

	emails := make([]models.Email, 0, len(ids))
	for email := range resultChan {
		emails = append(emails, email)
	}

	//Меня жутко бесит, что в го любую херню принято проверять. В данном случае проверяем, не закрыт ли уже контекст(ну мало ли, таймаут)
	//Хотя мне кажется эти строки не нужны
	if ctx.Err() != nil {
		return emails, fmt.Errorf("Операция прервана:%w", ctx.Err())
	}

	return emails, nil

}

func getEmailById(srv *gmail.Service, id string) (models.Email, error) {
	msg, err := srv.Users.Messages.Get("me", id).Format("full").Do()
	if err != nil {
		return models.Email{}, fmt.Errorf("Ошибка GET к gmail: %w", err)
	}

	subject := ""
	snippet := msg.Snippet

	for _, h := range msg.Payload.Headers {
		if h.Name == "Subject" {
			subject = h.Value
			break
		}
	}

	return models.Email{
		ID:      id,
		Subject: subject,
		Snippet: snippet,
	}, nil
}
