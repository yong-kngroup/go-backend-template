package postgres

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

const (
	gormTracingPluginName = "otel-gorm"
	gormSpanKeyPrefix     = "otel-gorm:span:"
)

type tracingPlugin struct {
	tracer trace.Tracer
}

func newTracingPlugin() gorm.Plugin {
	return &tracingPlugin{tracer: otel.Tracer("github.com/freeDog-wy/go-backend-template/internal/infra/postgres")}
}

func (p *tracingPlugin) Name() string { return gormTracingPluginName }

func (p *tracingPlugin) Initialize(db *gorm.DB) error {
	callbacks := []struct {
		register  func(string, func(*gorm.DB)) error
		operation string
	}{
		{db.Callback().Create().Before("gorm:create").Register, "CREATE"},
		{db.Callback().Create().After("gorm:create").Register, "CREATE"},
		{db.Callback().Query().Before("gorm:query").Register, "SELECT"},
		{db.Callback().Query().After("gorm:query").Register, "SELECT"},
		{db.Callback().Update().Before("gorm:update").Register, "UPDATE"},
		{db.Callback().Update().After("gorm:update").Register, "UPDATE"},
		{db.Callback().Delete().Before("gorm:delete").Register, "DELETE"},
		{db.Callback().Delete().After("gorm:delete").Register, "DELETE"},
		{db.Callback().Row().Before("gorm:row").Register, "ROW"},
		{db.Callback().Row().After("gorm:row").Register, "ROW"},
		{db.Callback().Raw().Before("gorm:raw").Register, "RAW"},
		{db.Callback().Raw().After("gorm:raw").Register, "RAW"},
	}
	for index, callback := range callbacks {
		if err := callback.register(fmt.Sprintf("%s:%d", gormTracingPluginName, index), p.callback(callback.operation, index%2 == 0)); err != nil {
			return err
		}
	}
	return nil
}

func (p *tracingPlugin) callback(operation string, before bool) func(*gorm.DB) {
	if before {
		return p.before(operation)
	}
	return p.after(operation)
}

func (p *tracingPlugin) before(operation string) func(*gorm.DB) {
	return func(db *gorm.DB) {
		if db == nil || db.Statement == nil {
			return
		}
		ctx := db.Statement.Context
		if ctx == nil {
			ctx = context.Background()
		}
		spanName := fmt.Sprintf("gorm.%s", strings.ToLower(operation))
		if table := strings.TrimSpace(db.Statement.Table); table != "" {
			spanName += " " + table
		}
		ctx, span := p.tracer.Start(ctx, spanName)
		db.Statement.Context = ctx
		db.InstanceSet(gormSpanKey(operation), span)
	}
}

func (p *tracingPlugin) after(operation string) func(*gorm.DB) {
	return func(db *gorm.DB) {
		if db == nil || db.Statement == nil {
			return
		}
		value, ok := db.InstanceGet(gormSpanKey(operation))
		if !ok {
			return
		}
		span, ok := value.(trace.Span)
		if !ok || span == nil {
			return
		}
		defer span.End()

		attrs := []attribute.KeyValue{
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", operation),
			attribute.Int64("db.rows_affected", db.RowsAffected),
		}
		if table := strings.TrimSpace(db.Statement.Table); table != "" {
			attrs = append(attrs, attribute.String("db.sql.table", table))
		}
		if sql := strings.TrimSpace(db.Statement.SQL.String()); sql != "" {
			attrs = append(attrs, attribute.String("db.query.text", sql))
		}
		span.SetAttributes(attrs...)
		if db.Error != nil {
			span.RecordError(db.Error)
			span.SetStatus(codes.Error, db.Error.Error())
			return
		}
		span.SetStatus(codes.Ok, "")
	}
}

func gormSpanKey(operation string) string { return gormSpanKeyPrefix + strings.ToLower(operation) }
