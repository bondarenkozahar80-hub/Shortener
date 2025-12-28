package repo

import (
	"context"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/wb-go/wbf/dbpg"
	"io/ioutil"
	"path/filepath"
	"time"
)

type Repository interface {
	MigrateUp(migrationsDir string) error
	MigrateDown(migrationsDir string) error
	CreateUrl(ctx context.Context, url UrlEntity) (int64, error)
	GetUrlByShort(ctx context.Context, short string) (*UrlEntity, error)
	CreateClick(ctx context.Context, click ClickEntity) error
	GetUrlAnalytics(ctx context.Context, short string) (*UrlAnalytics, error)
	GetUserAgentStats(ctx context.Context, short string) ([]UserAgentStat, error)
	GetAnalyticsByDay(ctx context.Context, short string, day time.Time) (*UrlAnalyticsByPeriod, error)
	GetAnalyticsByMonth(ctx context.Context, short string, month time.Time) (*UrlAnalyticsByPeriod, error)
	GetAnalyticsByField(ctx context.Context, short string, field string) ([]FieldStat, *AnalyticsPeriod, error)
}

type repository struct {
	db  *dbpg.DB
	log *zerolog.Logger
	ctx context.Context
}

func NewRepository(ctx context.Context, db *dbpg.DB, log *zerolog.Logger) (Repository, error) {
	if db == nil {
		return nil, fmt.Errorf("db cannot be nil")
	}
	if err := db.Master.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping DB: %w", err)
	}

	return &repository{
		db:  db,
		log: log,
		ctx: ctx,
	}, nil
}

func (r *repository) MigrateUp(migrationsDir string) error {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return fmt.Errorf("failed to read migration files: %w", err)
	}

	for _, file := range files {
		sqlBytes, err := ioutil.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}

		if _, err := r.db.ExecContext(r.ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", file, err)
		}
	}

	r.log.Info().Msgf("Migrations applied successfully from %s", migrationsDir)
	return nil
}

func (r *repository) MigrateDown(migrationsDir string) error {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.down.sql"))
	if err != nil {
		return fmt.Errorf("failed to read rollback files: %w", err)
	}

	for _, file := range files {
		sqlBytes, err := ioutil.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read rollback file %s: %w", file, err)
		}

		if _, err := r.db.ExecContext(r.ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("failed to rollback migration %s: %w", file, err)
		}
	}

	r.log.Info().Msgf("Migrations rolled back successfully from %s", migrationsDir)
	return nil
}

func (r *repository) CreateUrl(ctx context.Context, url UrlEntity) (int64, error) {
	query := `
		INSERT INTO urls (short, original, custom_alias, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	rows, err := r.db.QueryContext(ctx, query,
		url.Short,
		url.Original,
		url.CustomAlias,
		url.CreatedAt,
		url.ExpiresAt,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert url: %w", err)
	}
	defer rows.Close()

	var id int64
	if rows.Next() {
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("failed to scan returned id: %w", err)
		}
	} else {
		return 0, fmt.Errorf("no id returned after insert")
	}

	return id, nil
}

func (r *repository) GetUrlByShort(ctx context.Context, short string) (*UrlEntity, error) {
	query := `
		SELECT id, short, original, custom_alias, created_at, expires_at
		FROM urls
		WHERE short = $1 OR custom_alias = $1
		LIMIT 1
	`

	rows, err := r.db.QueryContext(ctx, query, short)
	if err != nil {
		return nil, fmt.Errorf("failed to query url by short: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		var url UrlEntity
		if err := rows.Scan(
			&url.ID,
			&url.Short,
			&url.Original,
			&url.CustomAlias,
			&url.CreatedAt,
			&url.ExpiresAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan url: %w", err)
		}
		return &url, nil
	}

	// ничего не найдено
	return nil, nil
}
func (r *repository) CreateClick(ctx context.Context, click ClickEntity) error {
	query := `
		INSERT INTO clicks (short, created_at, ip, browser, os, device, raw_ua, referer)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.ExecContext(ctx, query,
		click.Short,
		click.CreatedAt,
		click.IP,
		click.Browser,
		click.OS,
		click.Device,
		click.RawUA,
		click.Referer,
	)
	if err != nil {
		return fmt.Errorf("failed to insert click: %w", err)
	}

	return nil
}

func (r *repository) GetUserAgentStats(ctx context.Context, short string) ([]UserAgentStat, error) {
	rows, err := r.db.QueryContext(ctx, `
        SELECT 
            COALESCE(browser, 'Unknown') AS browser,
            COALESCE(os, 'Unknown') AS os,
            COALESCE(device, 'Unknown') AS device,
            COUNT(*) AS count
        FROM clicks
        WHERE short = $1
        GROUP BY browser, os, device
        ORDER BY count DESC;
    `, short)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []UserAgentStat
	for rows.Next() {
		var s UserAgentStat
		if err := rows.Scan(&s.Browser, &s.OS, &s.Device, &s.Count); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}

func (r *repository) GetUrlAnalytics(ctx context.Context, short string) (*UrlAnalytics, error) {
	// totalClicks
	rows, err := r.db.QueryContext(ctx, `SELECT COUNT(*) FROM clicks WHERE short = $1`, short)
	if err != nil {
		return nil, fmt.Errorf("failed to get total clicks: %w", err)
	}
	defer rows.Close()

	var totalClicks int64
	if rows.Next() {
		if err := rows.Scan(&totalClicks); err != nil {
			return nil, fmt.Errorf("failed to scan total clicks: %w", err)
		}
	}

	// uniqueIPs
	rows, err = r.db.QueryContext(ctx, `SELECT COUNT(DISTINCT ip) FROM clicks WHERE short = $1`, short)
	if err != nil {
		return nil, fmt.Errorf("failed to get unique IPs: %w", err)
	}
	defer rows.Close()

	var uniqueIPs int64
	if rows.Next() {
		if err := rows.Scan(&uniqueIPs); err != nil {
			return nil, fmt.Errorf("failed to scan unique IPs: %w", err)
		}
	}

	// period
	rows, err = r.db.QueryContext(ctx, `
		SELECT 
			COUNT(*) FILTER (WHERE created_at >= NOW() - INTERVAL '7 days') AS last_7_days,
			COUNT(*) FILTER (WHERE created_at >= NOW() - INTERVAL '30 days') AS last_30_days,
			COUNT(*) AS all_time
		FROM clicks
		WHERE short = $1
	`, short)
	if err != nil {
		return nil, fmt.Errorf("failed to get analytics period: %w", err)
	}
	defer rows.Close()

	var period AnalyticsPeriod
	if rows.Next() {
		if err := rows.Scan(&period.Last7Days, &period.Last30Days, &period.AllTime); err != nil {
			return nil, fmt.Errorf("failed to scan period: %w", err)
		}
	}

	// user agent stats
	rows, err = r.db.QueryContext(ctx, `
		SELECT 
			COALESCE(browser, 'Unknown') AS browser,
			COALESCE(os, 'Unknown') AS os,
			COALESCE(device, 'Unknown') AS device,
			COUNT(*) AS count
		FROM clicks
		WHERE short = $1
		GROUP BY browser, os, device
		ORDER BY count DESC
	`, short)
	if err != nil {
		return nil, fmt.Errorf("failed to get user agent stats: %w", err)
	}
	defer rows.Close()

	var stats []UserAgentStat
	for rows.Next() {
		var s UserAgentStat
		if err := rows.Scan(&s.Browser, &s.OS, &s.Device, &s.Count); err != nil {
			return nil, fmt.Errorf("failed to scan user agent stat: %w", err)
		}
		stats = append(stats, s)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration failed: %w", err)
	}

	return &UrlAnalytics{
		Short:       short,
		TotalClicks: totalClicks,
		UniqueIPs:   uniqueIPs,
		UserAgents:  stats,
		Period:      period,
	}, nil
}

// GetAnalyticsByDay возвращает статистику за конкретный день
func (r *repository) GetAnalyticsByDay(ctx context.Context, short string, day time.Time) (*UrlAnalyticsByPeriod, error) {
	start := day.Truncate(24 * time.Hour)
	end := start.Add(24 * time.Hour)

	// totalClicks
	rows, err := r.db.QueryContext(ctx,
		`SELECT COUNT(*) FROM clicks WHERE short=$1 AND created_at >= $2 AND created_at < $3`,
		short, start, end)
	if err != nil {
		r.log.Error().Msgf("failed to get total clicks by day for short=%s: %v", short, err)
		return nil, err
	}
	defer rows.Close()

	var totalClicks int64
	if rows.Next() {
		if err := rows.Scan(&totalClicks); err != nil {
			r.log.Error().Msgf("failed to scan total clicks by day for short=%s: %v", short, err)
			return nil, err
		}
	}

	// uniqueIPs
	rowsIPs, err := r.db.QueryContext(ctx,
		`SELECT COUNT(DISTINCT ip) FROM clicks WHERE short=$1 AND created_at >= $2 AND created_at < $3`,
		short, start, end)
	if err != nil {
		r.log.Error().Msgf("failed to get unique IPs by day for short=%s: %v", short, err)
		return nil, err
	}
	defer rowsIPs.Close()

	var uniqueIPs int64
	if rowsIPs.Next() {
		if err := rowsIPs.Scan(&uniqueIPs); err != nil {
			r.log.Error().Msgf("failed to scan unique IPs by day for short=%s: %v", short, err)
			return nil, err
		}
	}

	// агрегаты browser/os/device
	rowsStats, err := r.db.QueryContext(ctx,
		`SELECT COALESCE(browser,'Unknown'), COALESCE(os,'Unknown'), COALESCE(device,'Unknown'), COUNT(*)
		 FROM clicks
		 WHERE short=$1 AND created_at >= $2 AND created_at < $3
		 GROUP BY browser, os, device
		 ORDER BY COUNT(*) DESC`,
		short, start, end)
	if err != nil {
		r.log.Error().Msgf("failed to get stats by day for short=%s: %v", short, err)
		return nil, err
	}
	defer rowsStats.Close()

	var stats []UserAgentStat
	for rowsStats.Next() {
		var s UserAgentStat
		if err := rowsStats.Scan(&s.Browser, &s.OS, &s.Device, &s.Count); err != nil {
			r.log.Error().Msgf("failed to scan user agent stat by day for short=%s: %v", short, err)
			return nil, err
		}
		stats = append(stats, s)
	}

	return &UrlAnalyticsByPeriod{
		Short:       short,
		TotalClicks: totalClicks,
		UniqueIPs:   uniqueIPs,
		UserAgents:  stats,
	}, nil
}

// GetAnalyticsByMonth возвращает статистику за конкретный месяц
func (r *repository) GetAnalyticsByMonth(ctx context.Context, short string, month time.Time) (*UrlAnalyticsByPeriod, error) {
	start := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
	end := start.AddDate(0, 1, 0)

	// totalClicks
	rows, err := r.db.QueryContext(ctx,
		`SELECT COUNT(*) FROM clicks WHERE short=$1 AND created_at >= $2 AND created_at < $3`,
		short, start, end)
	if err != nil {
		r.log.Error().Msgf("failed to get total clicks by month for short=%s: %v", short, err)
		return nil, err
	}
	defer rows.Close()

	var totalClicks int64
	if rows.Next() {
		if err := rows.Scan(&totalClicks); err != nil {
			r.log.Error().Msgf("failed to scan total clicks by month for short=%s: %v", short, err)
			return nil, err
		}
	}

	// uniqueIPs
	rowsIPs, err := r.db.QueryContext(ctx,
		`SELECT COUNT(DISTINCT ip) FROM clicks WHERE short=$1 AND created_at >= $2 AND created_at < $3`,
		short, start, end)
	if err != nil {
		r.log.Error().Msgf("failed to get unique IPs by month for short=%s: %v", short, err)
		return nil, err
	}
	defer rowsIPs.Close()

	var uniqueIPs int64
	if rowsIPs.Next() {
		if err := rowsIPs.Scan(&uniqueIPs); err != nil {
			r.log.Error().Msgf("failed to scan unique IPs by month for short=%s: %v", short, err)
			return nil, err
		}
	}

	// агрегаты browser/os/device
	rowsStats, err := r.db.QueryContext(ctx,
		`SELECT COALESCE(browser,'Unknown'), COALESCE(os,'Unknown'), COALESCE(device,'Unknown'), COUNT(*)
		 FROM clicks
		 WHERE short=$1 AND created_at >= $2 AND created_at < $3
		 GROUP BY browser, os, device
		 ORDER BY COUNT(*) DESC`,
		short, start, end)
	if err != nil {
		r.log.Error().Msgf("failed to get stats by month for short=%s: %v", short, err)
		return nil, err
	}
	defer rowsStats.Close()

	var stats []UserAgentStat
	for rowsStats.Next() {
		var s UserAgentStat
		if err := rowsStats.Scan(&s.Browser, &s.OS, &s.Device, &s.Count); err != nil {
			r.log.Error().Msgf("failed to scan user agent stat by month for short=%s: %v", short, err)
			return nil, err
		}
		stats = append(stats, s)
	}

	return &UrlAnalyticsByPeriod{
		Short:       short,
		TotalClicks: totalClicks,
		UniqueIPs:   uniqueIPs,
		UserAgents:  stats,
	}, nil
}

// GetAnalyticsByField агрегирует по одному полю (browser/os/device) за всю историю
func (r *repository) GetAnalyticsByField(ctx context.Context, short string, field string) ([]FieldStat, *AnalyticsPeriod, error) {
	if field != "browser" && field != "os" && field != "device" {
		err := fmt.Errorf("unsupported field for aggregation: %s", field)
		r.log.Error().Msgf("%v", err)
		return nil, nil, err
	}

	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT COALESCE(%s,'Unknown'), COUNT(*) FROM clicks WHERE short=$1 GROUP BY %s ORDER BY COUNT(*) DESC`, field, field),
		short)
	if err != nil {
		r.log.Error().Msgf("failed to get field stats for short=%s, field=%s: %v", short, field, err)
		return nil, nil, err
	}
	defer rows.Close()

	var stats []FieldStat
	for rows.Next() {
		var s FieldStat
		if err := rows.Scan(&s.Value, &s.Count); err != nil {
			r.log.Error().Msgf("failed to scan field stat for short=%s, field=%s: %v", short, field, err)
			return nil, nil, err
		}
		stats = append(stats, s)
	}

	// период для всей истории
	rowsPeriod, err := r.db.QueryContext(ctx,
		`SELECT 
			COUNT(*) FILTER (WHERE created_at >= NOW() - INTERVAL '7 days') AS last_7_days,
			COUNT(*) FILTER (WHERE created_at >= NOW() - INTERVAL '30 days') AS last_30_days,
			COUNT(*) AS all_time
		FROM clicks
		WHERE short=$1`, short)
	if err != nil {
		r.log.Error().Msgf("failed to get analytics period for short=%s: %v", short, err)
		return nil, nil, err
	}
	defer rowsPeriod.Close()

	var period AnalyticsPeriod
	if rowsPeriod.Next() {
		if err := rowsPeriod.Scan(&period.Last7Days, &period.Last30Days, &period.AllTime); err != nil {
			r.log.Error().Msgf("failed to scan analytics period for short=%s: %v", short, err)
			return nil, nil, err
		}
	}

	return stats, &period, nil
}
