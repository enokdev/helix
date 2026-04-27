package data

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	helixconfig "github.com/enokdev/helix/config"
	"github.com/enokdev/helix/core"
	datagorm "github.com/enokdev/helix/data/gorm"
	"github.com/enokdev/helix/starter/internal/gomodutil"
)

const (
	dataEnabledKey     = "helix.starters.data.enabled"
	dataAutoMigrateKey = "helix.starters.data.auto-migrate"
	databaseURLKey     = "database.url"
	poolMaxOpenKey     = "database.pool.max-open"
	poolMaxIdleKey     = "database.pool.max-idle"
)

// Starter auto-configures the DB connection when gorm.io/driver/sqlite is available.
type Starter struct {
	cfg    helixconfig.Loader
	models []any
}

// Option configures a Starter.
type Option func(*Starter)

// WithAutoMigrateModels specifies models to auto-migrate on database start.
func WithAutoMigrateModels(models ...any) Option {
	return func(s *Starter) {
		s.models = append(s.models, models...)
	}
}

// New creates a Starter using the provided configuration loader.
func New(cfg helixconfig.Loader, opts ...Option) *Starter {
	s := &Starter{cfg: cfg}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Condition reports whether the data starter should be activated.
func (s *Starter) Condition() bool {
	goModPath, err := gomodutil.FindGoModPath()
	if err != nil {
		slog.Debug("data starter: go.mod not found", "error", err)
		return false
	}

	goMod, err := os.ReadFile(goModPath)
	if err != nil || !bytes.Contains(goMod, []byte("gorm.io/driver/sqlite")) {
		return false
	}

	if s.cfg == nil {
		return false
	}

	urlVal, ok := s.cfg.Lookup(databaseURLKey)
	if !ok || stringValue(urlVal) == "" {
		return false
	}

	enabledVal, ok := s.cfg.Lookup(dataEnabledKey)
	if !ok {
		return true
	}
	enabled, parsed := parseBool(enabledVal)
	return !parsed || enabled
}

// Configure registers DB components and a lifecycle into the container.
func (s *Starter) Configure(container *core.Container) error {
	if container == nil {
		return nil
	}

	lc := &databaseLifecycle{}

	if s.cfg != nil {
		urlVal, _ := s.cfg.Lookup(databaseURLKey)
		url := stringValue(urlVal)

		db, openErr := datagorm.OpenSQLite(url)
		if openErr != nil {
			lc.startErr = fmt.Errorf("data starter: open: %w", openErr)
		} else {
			lc.db = db

			if poolErr := configurePool(s.cfg, db); poolErr != nil {
				lc.startErr = poolErr
			}

			if lc.startErr == nil {
				for _, comp := range db.Components() {
					_ = container.Register(comp)
				}
			}
		}

		autoVal, _ := s.cfg.Lookup(dataAutoMigrateKey)
		if autoMigrate, parsed := parseBool(autoVal); parsed && autoMigrate {
			lc.autoMigrate = true
			lc.models = s.models
		}
	}

	_ = container.Register(lc)
	return nil
}

type databaseLifecycle struct {
	db          *datagorm.DB
	startErr    error
	autoMigrate bool
	models      []any
}

func (l *databaseLifecycle) OnStart() error {
	if l.startErr != nil {
		if l.db != nil {
			_ = l.db.Close()
			l.db = nil
		}
		return l.startErr
	}
	if err := l.db.Ping(); err != nil {
		return fmt.Errorf("data starter: ping: %w", err)
	}
	if l.autoMigrate && len(l.models) > 0 {
		if err := l.db.AutoMigrate(l.models...); err != nil {
			return fmt.Errorf("data starter: auto migrate: %w", err)
		}
	}
	return nil
}

func (l *databaseLifecycle) OnStop() error {
	if l.db == nil {
		return nil
	}
	if err := l.db.Close(); err != nil {
		return fmt.Errorf("data starter: close: %w", err)
	}
	return nil
}

// configurePool reads pool config and applies it to the DB.
// Negative or non-parsable values return a startup error.
func configurePool(cfg helixconfig.Loader, db *datagorm.DB) error {
	maxOpen, err := poolValue(cfg, poolMaxOpenKey)
	if err != nil {
		return err
	}
	maxIdle, err := poolValue(cfg, poolMaxIdleKey)
	if err != nil {
		return err
	}
	if maxOpen == 0 && maxIdle == 0 {
		return nil
	}
	return db.ConfigurePool(datagorm.ConnectionPoolConfig{
		MaxOpenConns: maxOpen,
		MaxIdleConns: maxIdle,
	})
}

// poolValue reads an integer pool config value.
// Absent or zero → 0, nil (no-op). Negative or non-parsable → error.
func poolValue(cfg helixconfig.Loader, key string) (int, error) {
	val, ok := cfg.Lookup(key)
	if !ok {
		return 0, nil
	}
	n, parsed := intValue(val)
	if !parsed {
		return 0, fmt.Errorf("data starter: %s: non-parsable value", key)
	}
	if n < 0 {
		return 0, fmt.Errorf("data starter: %s: negative value", key)
	}
	return n, nil
}

func parseBool(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes":
			return true, true
		case "false", "0", "no":
			return false, true
		}
	case int:
		return v != 0, true
	case int8:
		return v != 0, true
	case int16:
		return v != 0, true
	case int32:
		return v != 0, true
	case int64:
		return v != 0, true
	case uint:
		return v != 0, true
	case uint8:
		return v != 0, true
	case uint16:
		return v != 0, true
	case uint32:
		return v != 0, true
	case uint64:
		return v != 0, true
	}
	return false, false
}

func stringValue(value any) string {
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func intValue(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case uint:
		return int(v), true
	case uint8:
		return int(v), true
	case uint16:
		return int(v), true
	case uint32:
		return int(v), true
	case uint64:
		return int(v), true
	case float32:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return 0, false
}
