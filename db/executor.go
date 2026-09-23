package db

import (
	"context"
	"database/sql"

	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/fields"
	pg "github.com/go-jet/jet/v2/postgres"
	log "github.com/sirupsen/logrus"
)

// TranslationContext carries info the DB layer needs to handle translations.
type TranslationContext struct {
	EntityName string
	Fields     []TranslatableFieldInfo
	Langs      []string
	EntityID   interface{}
}

// TranslatableFieldInfo describes a single translatable field.
type TranslatableFieldInfo struct {
	FieldName string
}

type DBExecutor interface {
	List(
		log *log.Entry,
		table pg.Table,
		primaryKey pg.Column,
		moduleFields []fields.ModuleField,
		filterRegistry map[string]fields.ModuleFilterField,
		page int64,
		size int64,
		searchColumns []pg.Column,
		searchText string,
		filter map[string]string,
		where pg.BoolExpression,
		joins []actions.ModuleActionJoin,
		sort *actions.SortOption,
		tc *TranslationContext,
	) (result []interface{}, rowsCount int64, err error)
	View(
		log *log.Entry,
		table pg.Table,
		primaryKey pg.Column,
		moduleFields []fields.ModuleField,
		where pg.BoolExpression,
		joins []actions.ModuleActionJoin,
		tc *TranslationContext,
	) (interface{}, error)
	Add(log *log.Entry, table pg.Table, primaryKey pg.Column, moduleFields []fields.ModuleField, input map[string]interface{}, tc *TranslationContext) (interface{}, error)
	Update(log *log.Entry, table pg.Table, primaryKey pg.Column, moduleFields []fields.ModuleField, input map[string]interface{}, where pg.BoolExpression, tc *TranslationContext) (interface{}, error)
	Delete(log *log.Entry, table pg.Table, where pg.BoolExpression, tc *TranslationContext) error
	RawRequest(log *log.Entry, query string, params ...interface{}) (*sql.Rows, error)
	RawDB() *sql.DB
}

// WithRequestContext binds generated reads when the executor supports it.
// Existing third-party executors do not need to implement a new interface.
func WithRequestContext(executor DBExecutor, ctx context.Context) DBExecutor {
	if scoped, ok := executor.(interface {
		WithContext(context.Context) DBExecutor
	}); ok {
		return scoped.WithContext(ctx)
	}
	return executor
}
