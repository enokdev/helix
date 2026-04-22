package scaffold

const goModTemplate = `module {{ .ModulePath }}

go 1.21.0

require github.com/enokdev/helix v0.0.0
{{ .ExtraGoMod }}
{{ if .HelixReplacePath }}
replace github.com/enokdev/helix => {{ .HelixReplacePath }}
{{ end }}
`

const mainTemplate = `package main

import (
	"log"

	"github.com/enokdev/helix"
)

func main() {
	if err := helix.Run(helix.App{}); err != nil {
		log.Fatal(err)
	}
}
`

const applicationConfigTemplate = `app:
  name: {{ .Name }}
server:
  port: 8080
`

const repositoryTemplate = `package {{ .PackageName }}

import "github.com/enokdev/helix"

type {{ .TypeName }}Repository struct {
	helix.Repository
}
`

const serviceTemplate = `package {{ .PackageName }}

import "github.com/enokdev/helix"

type {{ .TypeName }}Service struct {
	helix.Service
	Repository *{{ .TypeName }}Repository ` + "`inject:\"true\"`" + `
}
`

const controllerTemplate = `package {{ .PackageName }}

import (
	"github.com/enokdev/helix"
	"github.com/enokdev/helix/web"
)

type {{ .TypeName }}Controller struct {
	helix.Controller
	Service *{{ .TypeName }}Service ` + "`inject:\"true\"`" + `
}

func (c *{{ .TypeName }}Controller) Index(ctx web.Context) error {
	return ctx.JSON(map[string]string{"module": "{{ .PackageName }}"})
}
`

const contextAPITemplate = `package {{ .PackageName }}

import (
	"context"
	"errors"
)

var ErrNotImplemented = errors.New("{{ .PackageName }}: context operation not implemented")

type {{ .TypeName }}ID string

type {{ .TypeName }} struct {
	ID {{ .TypeName }}ID
}

type Create{{ .TypeName }}Attrs struct {
	Name string
}

func Create{{ .TypeName }}(ctx context.Context, attrs Create{{ .TypeName }}Attrs) (*{{ .TypeName }}, error) {
	return new{{ .TypeName }}Service().Create{{ .TypeName }}(ctx, attrs)
}

func Get{{ .TypeName }}(ctx context.Context, id {{ .TypeName }}ID) (*{{ .TypeName }}, error) {
	return new{{ .TypeName }}Service().Get{{ .TypeName }}(ctx, id)
}

func new{{ .TypeName }}Service() *{{ .TypeName }}Service {
	return &{{ .TypeName }}Service{Repository: &{{ .TypeName }}Repository{}}
}
`

const contextRepositoryTemplate = `package {{ .PackageName }}

import "github.com/enokdev/helix"

type {{ .TypeName }}Repository struct {
	helix.Repository
}
`

const contextServiceTemplate = `package {{ .PackageName }}

import (
	"context"

	"github.com/enokdev/helix"
)

type {{ .TypeName }}Service struct {
	helix.Service
	Repository *{{ .TypeName }}Repository ` + "`inject:\"true\"`" + `
}

func (s *{{ .TypeName }}Service) Create{{ .TypeName }}(ctx context.Context, attrs Create{{ .TypeName }}Attrs) (*{{ .TypeName }}, error) {
	_ = attrs
	if ctx == nil {
		return nil, ErrNotImplemented
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, ErrNotImplemented
}

func (s *{{ .TypeName }}Service) Get{{ .TypeName }}(ctx context.Context, id {{ .TypeName }}ID) (*{{ .TypeName }}, error) {
	_ = id
	if ctx == nil {
		return nil, ErrNotImplemented
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, ErrNotImplemented
}
`

const contextControllerTemplate = `package {{ .PackageName }}

import (
	"github.com/enokdev/helix"
	"github.com/enokdev/helix/web"
)

type {{ .TypeName }}Controller struct {
	helix.Controller
	Service *{{ .TypeName }}Service ` + "`inject:\"true\"`" + `
}

func (c *{{ .TypeName }}Controller) Index(ctx web.Context) error {
	return ctx.JSON(map[string]string{"context": "{{ .PackageName }}"})
}
`
