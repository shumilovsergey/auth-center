package database

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

func New(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	if err := migrate(db); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		private_key BLOB NOT NULL,
		public_key TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	_, err := db.Exec(schema)
	return err
}

func (db *DB) GetUserByUsername(username string) (*User, error) {
	row := db.QueryRow(
		"SELECT id, username, password_hash, private_key, public_key, created_at FROM users WHERE username = ?",
		username,
	)

	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.PrivateKey, &u.PublicKey, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &u, nil
}

func (db *DB) CreateUser(username, passwordHash string, privateKey []byte, publicKey string) (*User, error) {
	result, err := db.Exec(
		"INSERT INTO users (username, password_hash, private_key, public_key) VALUES (?, ?, ?, ?)",
		username, passwordHash, privateKey, publicKey,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return db.getUserByID(id)
}

func (db *DB) UpdateUserPublicKey(userID int64, publicKey string) error {
	_, err := db.Exec("UPDATE users SET public_key = ? WHERE id = ?", publicKey, userID)
	return err
}

func (db *DB) getUserByID(id int64) (*User, error) {
	row := db.QueryRow(
		"SELECT id, username, password_hash, private_key, public_key, created_at FROM users WHERE id = ?",
		id,
	)

	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.PrivateKey, &u.PublicKey, &u.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	PrivateKey   []byte
	PublicKey    string
	CreatedAt    string
}
