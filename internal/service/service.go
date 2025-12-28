package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"github.com/mssola/useragent"
	"github.com/rs/zerolog"
	"github.com/wb-go/wbf/ginext"
	"github.com/wb-go/wbf/redis"
	"math/rand"
	"secondOne/internal/dto"
	"secondOne/internal/repo"
	"secondOne/pkg/validator"
	"time"
)

type Service interface {
	CreateUrl(ctx *ginext.Context)
	Redirect(ctx *ginext.Context)
	recordClick(ctx context.Context, short, ip, ua, referer string)
	ShowAnalytics(ctx *ginext.Context)
}

type service struct {
	repo repo.Repository
	log  *zerolog.Logger
	rdb  *redis.Client
}

func NewService(repo repo.Repository, logger *zerolog.Logger, rdb *redis.Client) Service {
	return &service{
		repo: repo,
		log:  logger,
		rdb:  rdb,
	}
}

func (s *service) CreateUrl(ctx *ginext.Context) {
	var req struct {
		Original    string     `json:"original" validate:"required,url"`
		CustomAlias *string    `json:"custom_alias,omitempty" validate:"omitempty,alphanum,min=3,max=30"`
		ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		s.log.Error().Msgf("Invalid request body: %v", err)
		dto.BadResponseError(ctx, dto.FieldBadFormat, "Invalid request body")
		return
	}

	if err := validator.Validate(ctx.Request.Context(), req); err != nil {
		dto.BadResponseError(ctx, dto.FieldIncorrect, err.Error())
		return
	}

	short := ""
	if req.CustomAlias != nil {
		short = *req.CustomAlias
	} else {
		short = generateShortCode(6)
	}

	urlEntity := repo.UrlEntity{
		Short:       short,
		Original:    req.Original,
		CustomAlias: req.CustomAlias,
		CreatedAt:   time.Now(),
		ExpiresAt:   req.ExpiresAt,
	}

	id, err := s.repo.CreateUrl(ctx.Request.Context(), urlEntity)
	if err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && req.CustomAlias != nil {
			dto.BadResponseError(ctx, dto.FieldIncorrect, "Custom alias already exists")
			return
		}
		s.log.Error().Msgf("Failed to create URL: %v", err)
		dto.InternalServerError(ctx)
		return
	}
	urlEntity.ID = id

	url := toServiceUrl(urlEntity)

	if s.rdb != nil {
		key := fmt.Sprintf("url:%s", url.Short)
		data, _ := json.Marshal(url)
		if err := s.rdb.Set(ctx, key, string(data)); err != nil {
			s.log.Warn().Msgf("Failed to cache URL in Redis: %v", err)
		}
	}

	dto.SuccessCreatedResponse(ctx, url)
}

func generateShortCode(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func (s *service) Redirect(ctx *ginext.Context) {
	short := ctx.Param("short_url")
	if short == "" {
		dto.FieldIncorrectError(ctx, "short_url")
		return
	}

	if s.rdb != nil {
		key := fmt.Sprintf("url:%s", short)
		if data, err := s.rdb.Get(ctx, key); err == nil {
			var url Url
			if err := json.Unmarshal([]byte(data), &url); err == nil {
				if url.ExpiresAt != nil && url.ExpiresAt.Before(time.Now()) {
					dto.FieldIncorrectError(ctx, "url")
					return
				}
				ip, ua, referer := getUserInfo(ctx)
				s.recordClick(ctx, url.Short, ip, ua, referer)
				ctx.Redirect(302, url.Original)
				return
			}
		}
	}

	// Получение из БД
	entity, err := s.repo.GetUrlByShort(ctx.Request.Context(), short)
	if err != nil {
		s.log.Error().Msgf("failed to get URL: %v", err)
		dto.ShortNotFoundError(ctx)
		return
	}
	if entity == nil {
		dto.ShortNotFoundError(ctx)
		return
	}
	if entity.ExpiresAt != nil && entity.ExpiresAt.Before(time.Now()) {
		dto.FieldIncorrectError(ctx, "url")
		return
	}

	ip, ua, referer := getUserInfo(ctx)
	s.recordClick(ctx, entity.Short, ip, ua, referer)

	ctx.Redirect(302, entity.Original)
}

func parseUserAgent(uaString string) (browser, os, device string) {
	if uaString == "" {
		return "Unknown", "Unknown", "Unknown"
	}

	ua := useragent.New(uaString)
	name, _ := ua.Browser()
	os = ua.OS()
	device = "Desktop"
	if ua.Mobile() {
		device = "Mobile"
	} else if ua.Bot() {
		device = "Bot"
	}

	return name, os, device
}

func (s *service) recordClick(ctx context.Context, short, ip, ua, referer string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.log.Error().Msgf("panic in recordClick: %v", r)
			}
		}()

		if ctx == nil {
			ctx = context.Background()
		}

		browser, os, device := parseUserAgent(ua)

		click := repo.ClickEntity{
			Short:     short,
			CreatedAt: time.Now(),
			IP:        &ip,
			RawUA:     &ua,
			Referer:   &referer,
			Browser:   &browser,
			OS:        &os,
			Device:    &device,
		}

		if err := s.repo.CreateClick(ctx, click); err != nil {
			s.log.Warn().Msgf("Failed to save click for short=%s: %v", short, err)
		}
	}()
}

func getUserInfo(ctx *ginext.Context) (ip, ua, referer string) {
	ip = ctx.ClientIP()
	ua = ctx.GetHeader("User-Agent")
	referer = ctx.GetHeader("Referer")
	return
}
func (s *service) ShowAnalytics(ctx *ginext.Context) {
	short := ctx.Param("short_url")
	if short == "" {
		dto.FieldIncorrectError(ctx, "short_url")
		return
	}

	// Проверка существования ссылки
	entity, err := s.repo.GetUrlByShort(ctx.Request.Context(), short)
	if err != nil || entity == nil {
		dto.ShortNotFoundError(ctx)
		return
	}

	var req struct {
		By    string `json:"by,omitempty"`    // "day", "month", "browser", "os", "device"
		Value string `json:"value,omitempty"` // дата "YYYY-MM-DD" или "YYYY-MM" для месяца
	}
	_ = ctx.ShouldBindJSON(&req)

	switch req.By {
	case "day":
		if req.Value == "" {
			dto.BadResponseError(ctx, dto.FieldIncorrect, "'value' must be specified for day analytics")
			return
		}
		day, err := time.Parse("2006-01-02", req.Value)
		if err != nil {
			dto.BadResponseError(ctx, dto.FieldIncorrect, "invalid date format, must be YYYY-MM-DD")
			return
		}

		data, err := s.repo.GetAnalyticsByDay(ctx.Request.Context(), short, day)
		if err != nil {
			dto.InternalServerError(ctx)
			return
		}
		dto.SuccessResponse(ctx, data)

	case "month":
		if req.Value == "" {
			dto.BadResponseError(ctx, dto.FieldIncorrect, "'value' must be specified for month analytics")
			return
		}
		month, err := time.Parse("2006-01", req.Value)
		if err != nil {
			dto.BadResponseError(ctx, dto.FieldIncorrect, "invalid month format, must be YYYY-MM")
			return
		}

		data, err := s.repo.GetAnalyticsByMonth(ctx.Request.Context(), short, month)
		if err != nil {
			dto.InternalServerError(ctx)
			return
		}
		dto.SuccessResponse(ctx, data)

	case "browser", "os", "device":
		data, period, err := s.repo.GetAnalyticsByField(ctx.Request.Context(), short, req.By)
		if err != nil {
			dto.InternalServerError(ctx)
			return
		}

		result := struct {
			Short  string               `json:"short"`
			Field  string               `json:"field"`
			Stats  []repo.FieldStat     `json:"stats"`
			Period repo.AnalyticsPeriod `json:"period"`
		}{
			Short:  short,
			Field:  req.By,
			Stats:  data,
			Period: *period,
		}

		dto.SuccessResponse(ctx, result)

	default:
		analytics, err := s.repo.GetUrlAnalytics(ctx.Request.Context(), short)
		if err != nil {
			dto.InternalServerError(ctx)
			return
		}
		dto.SuccessResponse(ctx, analytics)
	}
}
