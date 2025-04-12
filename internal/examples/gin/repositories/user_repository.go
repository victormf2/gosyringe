//go:generate mockgen -source=user_repository.go -destination=../mocks/user_repository.go -package=mocks
package repositories

import (
	"database/sql"
	"errors"
)

type User struct {
	ID    int64
	Name  string
	Email string
}

type IUserRepository interface {
	Create(user *User) error
	GetByID(id int64) (*User, error)
	Update(user *User) error
	Delete(id int64) error
}

type userRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) IUserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(user *User) error {
	query := `INSERT INTO users (name, email) VALUES (?, ?)`
	result, err := r.db.Exec(query, user.Name, user.Email)
	if err != nil {
		return err
	}
	user.ID, err = result.LastInsertId()
	return err
}

func (r *userRepository) GetByID(id int64) (*User, error) {
	query := `SELECT id, name, email FROM users WHERE id = ?`
	row := r.db.QueryRow(query, id)

	var user User
	err := row.Scan(&user.ID, &user.Name, &user.Email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &user, err
}

func (r *userRepository) Update(user *User) error {
	query := `UPDATE users SET name = ?, email = ? WHERE id = ?`
	_, err := r.db.Exec(query, user.Name, user.Email, user.ID)
	return err
}

func (r *userRepository) Delete(id int64) error {
	query := `DELETE FROM users WHERE id = ?`
	_, err := r.db.Exec(query, id)
	return err
}
