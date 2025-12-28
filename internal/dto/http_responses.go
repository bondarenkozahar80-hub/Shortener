package dto

import (
	"github.com/wb-go/wbf/ginext"
)

const (
	FieldBadFormat     = "FIELD_BADFORMAT"
	FieldIncorrect     = "FIELD_INCORRECT"
	ServiceUnavailable = "SERVICE_UNAVAILABLE"
	InternalError      = "Service is currently unavailable. Please try again later."

	ShortAlreadyExists = "SHORT_ALREADY_EXISTS"
	ShortNotFound      = "SHORT_NOT_FOUND"
)

type CreateShortRequest struct {
	URL    string `json:"url" validate:"required,url"`
	Custom string `json:"custom,omitempty" validate:"omitempty,min=3,max=30,tag"`
}

type ShortResponse struct {
	Short     string `json:"short"`
	ShortURL  string `json:"short_url"`
	Original  string `json:"original"`
	CreatedAt string `json:"createdAt"`
}

type AnalyticsResponse struct {
	Short       string         `json:"short"`
	Total       int            `json:"total"`
	ByDay       map[string]int `json:"byDay,omitempty"`
	ByMonth     map[string]int `json:"byMonth,omitempty"`
	ByUserAgent map[string]int `json:"byUserAgent,omitempty"`
}

type Response struct {
	Status string      `json:"status"`
	Error  *Error      `json:"error,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}

type Error struct {
	Code string `json:"code"`
	Desc string `json:"desc"`
}

func BadResponseError(c *ginext.Context, code, desc string) {
	c.JSON(400, Response{
		Status: "error",
		Error: &Error{
			Code: code,
			Desc: desc,
		},
	})
}

func InternalServerError(c *ginext.Context) {
	c.JSON(500, Response{
		Status: "error",
		Error: &Error{
			Code: ServiceUnavailable,
			Desc: InternalError,
		},
	})
}

func FieldBadFormatError(c *ginext.Context, fieldName string) {
	BadResponseError(c, FieldBadFormat, "Field '"+fieldName+"' has bad format")
}

func FieldIncorrectError(c *ginext.Context, fieldName string) {
	BadResponseError(c, FieldIncorrect, "Field '"+fieldName+"' is incorrect")
}

func ShortAlreadyExistsError(c *ginext.Context) {
	BadResponseError(c, ShortAlreadyExists, "Short alias already exists")
}

func ShortNotFoundError(c *ginext.Context) {
	BadResponseError(c, ShortNotFound, "Short link not found")
}

func SuccessResponse(c *ginext.Context, data interface{}) {
	c.JSON(200, Response{
		Status: "ok",
		Data:   data,
	})
}

func SuccessCreatedResponse(c *ginext.Context, data interface{}) {
	c.JSON(201, Response{
		Status: "ok",
		Data:   data,
	})
}
