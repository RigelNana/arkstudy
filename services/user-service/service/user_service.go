package service

import (
	"user-service/models"
	"user-service/repository"

	"github.com/google/uuid"
)

type UserService interface {
	Create(username, email, role, description string) (*models.User, error)
	GetByID(id uuid.UUID) (*models.User, error)
	GetByUsername(username string) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	List(limit, offset int) ([]*models.User, int64, error)
}

type UserServiceImpl struct{ repo repository.UserRepository }

func NewUserService(r repository.UserRepository) UserService { return &UserServiceImpl{repo: r} }

func (s *UserServiceImpl) Create(username, email, role, description string) (*models.User, error) {
	u := &models.User{Username: username, Email: email, Role: role, Description: description}
	if err := s.repo.Create(u); err != nil {
		return nil, err
	}
	return u, nil
}
func (s *UserServiceImpl) GetByID(id uuid.UUID) (*models.User, error) { return s.repo.GetByID(id) }
func (s *UserServiceImpl) GetByUsername(username string) (*models.User, error) {
	return s.repo.GetByUsername(username)
}
func (s *UserServiceImpl) GetByEmail(email string) (*models.User, error) {
	return s.repo.GetByEmail(email)
}
func (s *UserServiceImpl) List(limit, offset int) ([]*models.User, int64, error) {
	items, err := s.repo.List(limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.Count()
	return items, total, err
}
