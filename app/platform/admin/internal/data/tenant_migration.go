package data

import (
	"context"
	stdsql "database/sql"
	"fmt"

	"backend-service/app/platform/admin/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
)

var tenantUniqueColumns = map[string][]string{
	"users":    {"name", "email", "phone"},
	"roles":    {"name"},
	"posts":    {"name"},
	"depts":    {"name"},
	"projects": {"name", "code"},
}

func RunLegacyTenantBackfill(ctx context.Context, cfg *conf.Data, tenantID uint32, logger log.Logger) error {
	if tenantID == 0 {
		return fmt.Errorf("legacy tenant ID must be positive")
	}
	if cfg == nil || cfg.Database == nil {
		return fmt.Errorf("database config is required")
	}
	db, err := stdsql.Open(cfg.Database.Driver, cfg.Database.Source)
	if err != nil {
		return fmt.Errorf("opening database for tenant backfill: %w", err)
	}
	defer db.Close()

	if err := backfillTenantScope(ctx, db, tenantID); err != nil {
		return err
	}
	log.NewHelper(log.With(logger, "module", "tenant/backfill")).
		Infof("legacy Admin tenant data assigned to tenant=%d", tenantID)
	return nil
}

func backfillTenantScope(ctx context.Context, db *stdsql.DB, tenantID uint32) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting tenant backfill transaction: %w", err)
	}
	defer tx.Rollback()

	for table, columns := range tenantUniqueColumns {
		for _, column := range columns {
			query := fmt.Sprintf(
				"SELECT `%s`, COUNT(*) FROM `%s` WHERE `%s` IS NOT NULL AND `%s` <> '' GROUP BY `%s` HAVING COUNT(*) > 1 LIMIT 1",
				column, table, column, column, column,
			)
			var value string
			var count int
			err := tx.QueryRowContext(ctx, query).Scan(&value, &count)
			if err == nil {
				return fmt.Errorf("cannot backfill tenant data: %s.%s value %q occurs %d times", table, column, value, count)
			}
			if err != stdsql.ErrNoRows {
				return fmt.Errorf("checking %s.%s uniqueness: %w", table, column, err)
			}
		}
	}

	for table := range tenantUniqueColumns {
		query := fmt.Sprintf("UPDATE `%s` SET `tenant_id` = ? WHERE `tenant_id` <> ?", table)
		if _, err := tx.ExecContext(ctx, query, tenantID, tenantID); err != nil {
			return fmt.Errorf("backfilling %s tenant: %w", table, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing tenant backfill: %w", err)
	}
	return nil
}
