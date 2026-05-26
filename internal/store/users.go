package store

import (
	"database/sql"
	"fmt"
	"time"
)

type User struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	Role        string     `json:"role"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

type UserWithPassword struct {
	User
	PasswordHash string
}

func (db *DB) CountUsers() (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func (db *DB) CreateUser(u *UserWithPassword) error {
	_, err := db.Exec(`
		INSERT INTO users (id, username, role, password_hash)
		VALUES (?, ?, ?, ?)`,
		u.ID, u.Username, u.Role, u.PasswordHash,
	)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (db *DB) ListUsers() ([]*User, error) {
	rows, err := db.Query(`
		SELECT id, username, role, created_at, updated_at, last_login_at
		FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (db *DB) GetUser(id string) (*User, error) {
	row := db.QueryRow(`
		SELECT id, username, role, created_at, updated_at, last_login_at
		FROM users WHERE id = ?`, id)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (db *DB) GetUserByUsername(username string) (*UserWithPassword, error) {
	u := &UserWithPassword{}
	var lastLogin sql.NullTime
	err := db.QueryRow(`
		SELECT id, username, role, password_hash, created_at, updated_at, last_login_at
		FROM users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &u.Role, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt, &lastLogin)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastLogin.Valid {
		u.LastLoginAt = &lastLogin.Time
	}
	return u, nil
}

func (db *DB) UpdateUser(id, role, passwordHash string) error {
	if passwordHash != "" {
		_, err := db.Exec(`UPDATE users SET role = ?, password_hash = ? WHERE id = ?`, role, passwordHash, id)
		return err
	}
	_, err := db.Exec(`UPDATE users SET role = ? WHERE id = ?`, role, id)
	return err
}

func (db *DB) DeleteUser(id string) error {
	_, err := db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func (db *DB) TouchUserLogin(id string) error {
	_, err := db.Exec(`UPDATE users SET last_login_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

func (db *DB) CreateSession(tokenHash, userID string, expiresAt time.Time) error {
	_, err := db.Exec(`
		INSERT INTO sessions (token_hash, user_id, expires_at)
		VALUES (?, ?, ?)`,
		tokenHash, userID, expiresAt,
	)
	return err
}

func (db *DB) GetSessionUser(tokenHash string) (*User, error) {
	row := db.QueryRow(`
		SELECT u.id, u.username, u.role, u.created_at, u.updated_at, u.last_login_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.expires_at > CURRENT_TIMESTAMP`, tokenHash)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (db *DB) DeleteSession(tokenHash string) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (db *DB) DeleteUserSessions(userID string) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

func (db *DB) CleanupExpiredSessions() error {
	_, err := db.Exec(`DELETE FROM sessions WHERE expires_at <= CURRENT_TIMESTAMP`)
	return err
}

type userScanner interface {
	Scan(dest ...interface{}) error
}

func scanUser(row userScanner) (*User, error) {
	u := &User{}
	var lastLogin sql.NullTime
	if err := row.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt, &u.UpdatedAt, &lastLogin); err != nil {
		return nil, err
	}
	if lastLogin.Valid {
		u.LastLoginAt = &lastLogin.Time
	}
	return u, nil
}
