package models

import "time"

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	PrivateKey   []byte
	PublicKey    string
	CreatedAt    time.Time
}
