package db

import (
	"context"
	"database/sql"
	"fmt"

	"gorm.io/gorm"

	"go-stock/backend/tenant"
)

// switchPool 把全局 Dao 的 SQL 发到当前 Bind 的用户库。未绑定则走 fallback（网页版 shell 库）。
type switchPool struct {
	fallback gorm.ConnPool
}

func (p *switchPool) current() gorm.ConnPool {
	if rt := tenant.Current(); rt != nil && rt.DB != nil && rt.DB.ConnPool != nil {
		return rt.DB.ConnPool
	}
	return p.fallback
}

func (p *switchPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return p.current().PrepareContext(ctx, query)
}

func (p *switchPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return p.current().ExecContext(ctx, query, args...)
}

func (p *switchPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return p.current().QueryContext(ctx, query, args...)
}

func (p *switchPool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return p.current().QueryRowContext(ctx, query, args...)
}

func (p *switchPool) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	cur := p.current()
	if beginner, ok := cur.(gorm.ConnPoolBeginner); ok {
		return beginner.BeginTx(ctx, opts)
	}
	if tb, ok := cur.(gorm.TxBeginner); ok {
		tx, err := tb.BeginTx(ctx, opts)
		if err != nil {
			return nil, err
		}
		return tx, nil
	}
	return nil, fmt.Errorf("conn pool does not support transactions")
}

func (p *switchPool) GetDBConn() (*sql.DB, error) {
	cur := p.current()
	if sqldb, ok := cur.(*sql.DB); ok {
		return sqldb, nil
	}
	if getter, ok := cur.(gorm.GetDBConnector); ok {
		return getter.GetDBConn()
	}
	if p.fallback != nil {
		if sqldb, ok := p.fallback.(*sql.DB); ok {
			return sqldb, nil
		}
		if getter, ok := p.fallback.(gorm.GetDBConnector); ok {
			return getter.GetDBConn()
		}
	}
	return nil, gorm.ErrInvalidDB
}

func wrapSwitchingConnPool(gdb *gorm.DB) {
	if gdb == nil || gdb.Config == nil {
		return
	}
	if _, ok := gdb.ConnPool.(*switchPool); ok {
		return
	}
	sw := &switchPool{fallback: gdb.ConnPool}
	gdb.Config.ConnPool = sw
	gdb.ConnPool = sw
	if gdb.Statement != nil {
		gdb.Statement.ConnPool = sw
	}
}
