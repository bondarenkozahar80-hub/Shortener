package service

import (
	"secondOne/internal/repo"
	"time"
)

type Url struct {
	ID          int64      `json:"id"`
	Short       string     `json:"short"`
	Original    string     `json:"original"`
	CustomAlias *string    `json:"custom_alias,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

type Click struct {
	ID        int64     `json:"id"`
	Short     string    `json:"short"`
	CreatedAt time.Time `json:"created_at"`
	IP        *string   `json:"ip,omitempty"`
	Referer   *string   `json:"referer,omitempty"`
	Browser   *string   `json:"browser,omitempty"`
	OS        *string   `json:"os,omitempty"`
	Device    *string   `json:"device,omitempty"`
	RawUA     *string   `json:"raw_ua,omitempty"`
}

type UrlAnalytics struct {
	Short       string          `json:"short"`
	TotalClicks int64           `json:"total_clicks"`
	UniqueIPs   int64           `json:"unique_ips"`
	UserAgents  []UserAgentStat `json:"user_agents"`
	Period      AnalyticsPeriod `json:"period"`
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

func toServiceUrl(e repo.UrlEntity) Url {
	return Url{
		ID:          e.ID,
		Short:       e.Short,
		Original:    e.Original,
		CustomAlias: e.CustomAlias,
		CreatedAt:   e.CreatedAt,
		ExpiresAt:   e.ExpiresAt,
	}
}

type AnalyticsRequest struct {
	By    string `json:"by,omitempty"`
	Value string `json:"value,omitempty"`
}
