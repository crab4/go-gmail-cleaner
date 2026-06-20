package models

type Email struct {
	ID      string
	Subject string
	Snippet string //Короткий фрагмент письма, попробуем определять письмо по нему
}
