package data

import (
	"fmt"
	"reflect"
)

// Operator describes a portable repository filter comparison.
type Operator string

const (
	// OperatorEqual matches records where a field equals the provided value.
	OperatorEqual Operator = "eq"
	// OperatorNotEqual matches records where a field differs from the provided value.
	OperatorNotEqual Operator = "ne"
	// OperatorGreaterThan matches records where a field is greater than the provided value.
	OperatorGreaterThan Operator = "gt"
	// OperatorGreaterThanOrEqual matches records where a field is greater than or equal to the value.
	OperatorGreaterThanOrEqual Operator = "gte"
	// OperatorLessThan matches records where a field is less than the provided value.
	OperatorLessThan Operator = "lt"
	// OperatorLessThanOrEqual matches records where a field is less than or equal to the value.
	OperatorLessThanOrEqual Operator = "lte"
	// OperatorContains matches records where a field contains the provided value.
	OperatorContains Operator = "contains"
	// OperatorIn matches records where a field belongs to the provided value set.
	OperatorIn Operator = "in"
	// OperatorIsNull matches records where a field is null.
	OperatorIsNull Operator = "is_null"
	// OperatorIsNotNull matches records where a field is not null.
	OperatorIsNotNull Operator = "is_not_null"
)

// LogicalOperator describes how filter conditions are combined.
type LogicalOperator string

const (
	// LogicalDefault combines conditions with AND when no operator is provided.
	LogicalDefault LogicalOperator = ""
	// LogicalAnd requires every condition to match.
	LogicalAnd LogicalOperator = "and"
	// LogicalOr requires at least one condition to match.
	LogicalOr LogicalOperator = "or"
)

// Condition describes one ORM-neutral field comparison.
type Condition struct {
	Field    string
	Operator Operator
	Value    any
}

// Filter describes ORM-neutral query conditions.
type Filter struct {
	Conditions []Condition
	Logic      LogicalOperator
}

// NewFilter builds and validates a portable repository filter.
func NewFilter(logic LogicalOperator, conditions ...Condition) (Filter, error) {
	filter := Filter{
		Conditions: append([]Condition(nil), conditions...),
		Logic:      logic,
	}
	if err := filter.Validate(); err != nil {
		return Filter{}, err
	}
	return filter, nil
}

// Validate returns an error if the filter cannot be translated safely by adapters.
func (f Filter) Validate() error {
	if !f.Logic.valid() {
		return fmt.Errorf("data: validate filter logic %q: %w", f.Logic, ErrInvalidFilter)
	}
	if f.Logic != LogicalDefault && len(f.Conditions) == 0 {
		return fmt.Errorf("data: validate filter: logic %q requires at least one condition: %w", f.Logic, ErrInvalidFilter)
	}

	for _, condition := range f.Conditions {
		if err := condition.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate returns an error if the condition cannot be translated safely by adapters.
func (c Condition) Validate() error {
	if c.Field == "" {
		return fmt.Errorf("data: validate filter condition field: %w", ErrInvalidFilter)
	}
	if !c.Operator.valid() {
		return fmt.Errorf("data: validate filter operator %q: %w", c.Operator, ErrInvalidFilter)
	}
	if !c.requiresValue() {
		return nil
	}
	if c.Value == nil {
		return fmt.Errorf("data: validate filter condition %s value: %w", c.Field, ErrInvalidFilter)
	}
	rv := reflect.ValueOf(c.Value)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Chan, reflect.Interface, reflect.Map, reflect.Slice:
		if rv.IsNil() {
			return fmt.Errorf("data: validate filter condition %s value: %w", c.Field, ErrInvalidFilter)
		}
	}
	if c.Operator == OperatorIn {
		switch rv.Kind() {
		case reflect.Slice, reflect.Array:
			if rv.Len() == 0 {
				return fmt.Errorf("data: validate filter condition %s: OperatorIn requires a non-empty slice: %w", c.Field, ErrInvalidFilter)
			}
		default:
			return fmt.Errorf("data: validate filter condition %s: OperatorIn requires a slice value: %w", c.Field, ErrInvalidFilter)
		}
	}
	return nil
}

func (c Condition) requiresValue() bool {
	return c.Operator != OperatorIsNull && c.Operator != OperatorIsNotNull
}

func (o Operator) valid() bool {
	switch o {
	case OperatorEqual,
		OperatorNotEqual,
		OperatorGreaterThan,
		OperatorGreaterThanOrEqual,
		OperatorLessThan,
		OperatorLessThanOrEqual,
		OperatorContains,
		OperatorIn,
		OperatorIsNull,
		OperatorIsNotNull:
		return true
	default:
		return false
	}
}

func (o LogicalOperator) valid() bool {
	switch o {
	case LogicalDefault, LogicalAnd, LogicalOr:
		return true
	default:
		return false
	}
}
