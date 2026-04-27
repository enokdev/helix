package main

import (
	"log"
	"net/http"
	"strconv"
	"sync"

	helix "github.com/enokdev/helix"
	"github.com/enokdev/helix/config"
	"github.com/enokdev/helix/core"
	"github.com/enokdev/helix/starter"
	webstarter "github.com/enokdev/helix/starter/web"
	"github.com/enokdev/helix/web"
)

type appConfig struct {
	Server struct {
		Port int `mapstructure:"port"`
	} `mapstructure:"server"`
	App struct {
		Name string `mapstructure:"name"`
	} `mapstructure:"app"`
}

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

	Repo *UserRepository `inject:"true"`
}

func NewUserService(repo *UserRepository) *UserService {
	return &UserService{Repo: repo}
}

func (s *UserService) List() []User {
	return s.Repo.FindAll()
}

func (s *UserService) Get(id int) (User, bool) {
	return s.Repo.FindByID(id)
}

func (s *UserService) Create(input userInput) User {
	return s.Repo.Save(input)
}

func (s *UserService) Update(id int, input userInput) (User, bool) {
	return s.Repo.Update(id, input)
}

func (s *UserService) Delete(id int) bool {
	return s.Repo.Delete(id)
}

type UserController struct {
	helix.Controller

	Service *UserService `inject:"true"`
}

func NewUserController(svc *UserService) *UserController {
	return &UserController{Service: svc}
}

func (c *UserController) Index() []User {
	return c.Service.List()
}

func (c *UserController) Show(ctx web.Context) (User, error) {
	id, err := userID(ctx)
	if err != nil {
		return User{}, err
	}
	user, ok := c.Service.Get(id)
	if !ok {
		return User{}, notFound()
	}
	return user, nil
}

func (c *UserController) Create(ctx web.Context, input userInput) (User, error) {
	user := c.Service.Create(input)
	ctx.Status(http.StatusCreated)
	return user, nil
}

func (c *UserController) Update(ctx web.Context, input userInput) (User, error) {
	id, err := userID(ctx)
	if err != nil {
		return User{}, err
	}
	user, ok := c.Service.Update(id, input)
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
	if !c.Service.Delete(id) {
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

func (e *httpError) Error() string      { return e.message }
func (e *httpError) StatusCode() int    { return e.status }
func (e *httpError) ErrorType() string  { return e.errType }
func (e *httpError) ErrorCode() string  { return e.code }
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

func loadConfig() (appConfig, error) {
	loader := config.NewLoader(
		config.WithConfigPaths("examples/crud-api/config", "config"),
		config.WithDefaults(map[string]any{
			"server.port": 8080,
			"app.name":    "helix-crud-api",
		}),
	)

	var cfg appConfig
	if err := loader.Load(&cfg); err != nil {
		return appConfig{}, err
	}
	return cfg, nil
}

func newServer() (web.HTTPServer, error) {
	container := core.NewContainer(core.WithResolver(core.NewReflectResolver()))
	for _, component := range []any{
		NewUserRepository(),
		&UserService{},
		&UserController{},
	} {
		if err := container.Register(component); err != nil {
			return nil, err
		}
	}

	var ctrl *UserController
	if err := container.Resolve(&ctrl); err != nil {
		return nil, err
	}

	server := web.NewServer()
	if err := web.RegisterController(server, ctrl); err != nil {
		return nil, err
	}
	return server, nil
}

func main() {
	loader := config.NewLoader(
		config.WithConfigPaths("examples/crud-api/config", "config"),
		config.WithDefaults(map[string]any{
			"server.port": 8080,
			"app.name":    "helix-crud-api",
		}),
	)

	if err := helix.Run(helix.App{
		Starters: []starter.Entry{
			{Name: "web", Order: starter.OrderWeb, Starter: webstarter.New(loader)},
		},
		Components: []any{
			NewUserRepository(),
			&UserService{},
			&UserController{},
		},
	}); err != nil {
		log.Fatal(err)
	}
}
