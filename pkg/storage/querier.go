package storage

import (
	"context"
	"fmt"
)

// Querier interface for database queries (matches original tools.Querier)
type Querier interface {
	Query(ctx context.Context, script string) (*QueryResult, error)
	QueryRaw(ctx context.Context, script string) (map[string]any, error)
}

// AnyToString converts any type to string
func AnyToString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	if s, ok := v.([]byte); ok {
		return string(s)
	}
	return fmt.Sprintf("%v", v)
}
