package main

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	helix "github.com/enokdev/helix"
	"github.com/enokdev/helix/web"
)

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type userInput struct {
	Name  string `json:"name"  validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

type UserRepository struct {
	helix.Repository

	mu     sync.Mutex
	nextID int
	users  map[int]User
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		nextID: 1,
		users:  make(map[int]User),
	}
}

func (r *UserRepository) FindAll() []User {
	r.mu.Lock()
	defer r.mu.Unlock()

	users := make([]User, 0, len(r.users))
	for id := 1; id < r.nextID; id++ {
		user, ok := r.users[id]
		if ok {
			users = append(users, user)
		}
	}
	return users
}

func (r *UserRepository) FindByID(id int) (User, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, ok := r.users[id]
	return user, ok
}

func (r *UserRepository) Save(input userInput) User {
	r.mu.Lock()
	defer r.mu.Unlock()

	user := User{ID: r.nextID, Name: input.Name, Email: input.Email}
	r.users[user.ID] = user
	r.nextID++
	return user
}

func (r *UserRepository) Update(id int, input userInput) (User, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.users[id]; !ok {
		return User{}, false
	}
	user := User{ID: id, Name: input.Name, Email: input.Email}
	r.users[id] = user
	return user, true
}

func (r *UserRepository) Delete(id int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.users[id]; !ok {
		return false
	}
	delete(r.users, id)
	return true
}

type UserService struct {
	helix.Service

	repo *UserRepository
}

func NewUserService(repo *UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) List() []User {
	return s.repo.FindAll()
}

func (s *UserService) Get(id int) (User, bool) {
	return s.repo.FindByID(id)
}

func (s *UserService) Create(input userInput) User {
	return s.repo.Save(input)
}

func (s *UserService) Update(id int, input userInput) (User, bool) {
	return s.repo.Update(id, input)
}

func (s *UserService) Delete(id int) bool {
	return s.repo.Delete(id)
}

type UserController struct {
	helix.Controller

	service *UserService
}

func NewUserController(svc *UserService) *UserController {
	return &UserController{service: svc}
}

func (c *UserController) Index() []User {
	return c.service.List()
}

func (c *UserController) Show(ctx web.Context) (User, error) {
	id, err := userID(ctx)
	if err != nil {
		return User{}, err
	}
	user, ok := c.service.Get(id)
	if !ok {
		return User{}, notFound()
	}
	return user, nil
}

func (c *UserController) Create(input userInput) User {
	return c.service.Create(input)
}

func (c *UserController) Update(ctx web.Context, input userInput) (User, error) {
	id, err := userID(ctx)
	if err != nil {
		return User{}, err
	}
	user, ok := c.service.Update(id, input)
	if !ok {
		return User{}, notFound()
	}
	return user, nil
}

func (c *UserController) Delete(ctx web.Context) error {
	id, err := userID(ctx)
	if err != nil {
		return err
	}
	if !c.service.Delete(id) {
		return notFound()
	}
	ctx.Status(http.StatusNoContent)
	return nil
}

func newServer() (web.HTTPServer, error) {
	server := web.NewServer()
	if err := web.RegisterController(server, NewUserController()); err != nil {
		return nil, err
	}
	return server, nil
}

func main() {
	server, err := newServer()
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(server.Start(":8080"))
}

func userID(ctx web.Context) (int, error) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return 0, errors.New("invalid user id")
	}
	return id, nil
}

func notFound(ctx web.Context) error {
	ctx.Status(http.StatusNotFound)
	return errors.New("user not found")
}
