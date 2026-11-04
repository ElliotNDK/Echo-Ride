package response

import "github.com/labstack/echo/v5"

type Response struct {
	StatusCode int         `json:"status_code"`
	Message    string      `json:"message"`
	Data       interface{} `json:"data,omitempty"`
	ErrorKey   string      `json:"error_key,omitempty"`
	TraceID    string      `json:"trace_id,omitempty"`
}

func WriteSuccess(e *echo.Context, statusCode int, data interface{}, msg string) error {
	return e.JSON(statusCode, Response{
		StatusCode: statusCode,
		Message:    msg,
		Data:       data,
	})
}
