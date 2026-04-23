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

// httpError carries HTTP status and structured metadata for writeErrorResponse.
type httpError struct {
	status  int
	errType string
	code    string
	message string
}

func (e *httpError) Error() string     { return e.message }
func (e *httpError) StatusCode() int   { return e.status }
func (e *httpError) ErrorType() string { return e.errType }
func (e *httpError) ErrorCode() string { return e.code }
func (e *httpError) ErrorField() string { return "" }

func notFound() error {
	return &httpError{status: http.StatusNotFound, errType: "NotFound", code: "NOT_FOUND", message: "user not found"}
}

func userID(ctx web.Context) (int, error) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return 0, &httpError{status: http.StatusBadRequest, errType: "BadRequest", code: "BAD_REQUEST", message: "invalid user id"}
	}
	return id, nil
}

// appServer wraps HTTPServer to participate in the Helix lifecycle.
type appServer struct {
	helix.Component

	server web.HTTPServer
	addr   string
}

func (s *appServer) OnStart() error {
	return s.server.Start(s.addr)
}

func (s *appServer) OnStop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.server.Stop(ctx)
}

func newServer() (web.HTTPServer, error) {
	repo := NewUserRepository()
	svc := NewUserService(repo)
	ctrl := NewUserController(svc)
	server := web.NewServer()
	if err := web.RegisterController(server, ctrl); err != nil {
		return nil, err
	}
	return server, nil
}

func main() {
	server, err := newServer()
	if err != nil {
		log.Fatal(err)
	}
	if err := helix.Run(helix.App{
		Components: []any{&appServer{server: server, addr: ":8080"}},
	}); err != nil {
		log.Fatal(err)
	}
}
