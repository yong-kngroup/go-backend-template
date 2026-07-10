package database

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
	return &tracingPlugin{
		tracer: otel.Tracer("github.com/freeDog-wy/go-backend-template/internal/infra/database"),
	}
}

func (p *tracingPlugin) Name() string {
	return gormTracingPluginName
}

func (p *tracingPlugin) Initialize(db *gorm.DB) error {
	if err := db.Callback().Create().Before("gorm:create").Register(gormTracingPluginName+":before_create", p.before("CREATE")); err != nil {
		return err
	}
	if err := db.Callback().Create().After("gorm:create").Register(gormTracingPluginName+":after_create", p.after("CREATE")); err != nil {
		return err
	}
	if err := db.Callback().Query().Before("gorm:query").Register(gormTracingPluginName+":before_query", p.before("SELECT")); err != nil {
		return err
	}
	if err := db.Callback().Query().After("gorm:query").Register(gormTracingPluginName+":after_query", p.after("SELECT")); err != nil {
		return err
	}
	if err := db.Callback().Update().Before("gorm:update").Register(gormTracingPluginName+":before_update", p.before("UPDATE")); err != nil {
		return err
	}
	if err := db.Callback().Update().After("gorm:update").Register(gormTracingPluginName+":after_update", p.after("UPDATE")); err != nil {
		return err
	}
	if err := db.Callback().Delete().Before("gorm:delete").Register(gormTracingPluginName+":before_delete", p.before("DELETE")); err != nil {
		return err
	}
	if err := db.Callback().Delete().After("gorm:delete").Register(gormTracingPluginName+":after_delete", p.after("DELETE")); err != nil {
		return err
	}
	if err := db.Callback().Row().Before("gorm:row").Register(gormTracingPluginName+":before_row", p.before("ROW")); err != nil {
		return err
	}
	if err := db.Callback().Row().After("gorm:row").Register(gormTracingPluginName+":after_row", p.after("ROW")); err != nil {
		return err
	}
	if err := db.Callback().Raw().Before("gorm:raw").Register(gormTracingPluginName+":before_raw", p.before("RAW")); err != nil {
		return err
	}
	if err := db.Callback().Raw().After("gorm:raw").Register(gormTracingPluginName+":after_raw", p.after("RAW")); err != nil {
		return err
	}
	return nil
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

func gormSpanKey(operation string) string {
	return gormSpanKeyPrefix + strings.ToLower(operation)
}
