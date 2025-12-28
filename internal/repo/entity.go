package repo

import "time"

type UrlEntity struct {
	ID          int64      `db:"id"`
	Short       string     `db:"short"`
	Original    string     `db:"original"`
	CustomAlias *string    `db:"custom_alias"`
	CreatedAt   time.Time  `db:"created_at"`
	ExpiresAt   *time.Time `db:"expires_at"`
}

type ClickEntity struct {
	ID        int64     `db:"id"`
	Short     string    `db:"short"`
	CreatedAt time.Time `db:"created_at"`
	IP        *string   `db:"ip"`
	Browser   *string   `db:"browser"`
	OS        *string   `db:"os"`
	Device    *string   `db:"device"`
	RawUA     *string   `db:"raw_ua"`
	Referer   *string   `db:"referer"`
}

type UrlAnalytics struct {
	Short       string          `json:"short"`
	TotalClicks int64           `json:"total_clicks"`
	UniqueIPs   int64           `json:"unique_ips"`
	UserAgents  []UserAgentStat `json:"user_agents"`
	Period      AnalyticsPeriod `json:"period"`
}

type UrlAnalyticsByPeriod struct {
	Short       string          `json:"short"`
	TotalClicks int64           `json:"total_clicks"`
	UniqueIPs   int64           `json:"unique_ips"`
	UserAgents  []UserAgentStat `json:"user_agents"`
}

type UserAgentStat struct {
	Browser string `json:"browser"`
	OS      string `json:"os"`
	Device  string `json:"device"`
	Count   int64  `json:"count"`
}

type AnalyticsPeriod struct {
	Last7Days  int64 `json:"last_7_days"`
	Last30Days int64 `json:"last_30_days"`
	AllTime    int64 `json:"all_time"`
}

type FieldStat struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}
