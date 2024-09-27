package healthcheck

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"go.uber.org/zap"
)

type Postgres struct {
	Host     string
	Port     string
	DB       string
	Username string
	Password string
}

func GetTables(ctx context.Context, logger *zap.Logger) error {
	config := Postgres{
		Host:     os.Getenv("POSTGRESQL_HOST"),
		Port:     os.Getenv("POSTGRESQL_PORT"),
		DB:       os.Getenv("POSTGRESQL_DB"),
		Username: os.Getenv("POSTGRESQL_USERNAME"),
		Password: os.Getenv("POSTGRESQL_PASSWORD"),
	}

	logger.Info(
		"parameters",
		zap.String("POSTGRESQL_HOST", config.Host),
		zap.String("POSTGRESQL_PORT", config.Port),
		zap.String("POSTGRESQL_DB", config.DB),
		zap.String("POSTGRESQL_USERNAME", config.Username),
		zap.String("POSTGRESQL_PASSWORD", config.Password),
	)

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		config.Username,
		config.Password,
		config.Host,
		config.Port,
		config.DB,
	)

	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	db := bun.NewDB(sqldb, pgdialect.New())

	var tables []string
	err := db.NewSelect().
		ColumnExpr("table_name").
		TableExpr("information_schema.tables").
		Where("table_schema = 'public'").
		Where("table_type = 'BASE TABLE'"). //
		Scan(ctx, &tables)
	if err != nil {
		return err
	}
	for _, table := range tables {
		log.Println(table)
	}
	return nil
}
