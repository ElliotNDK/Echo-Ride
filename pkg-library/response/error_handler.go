package response

import (
	"echo-ride/pkg-library/errs"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"
)

func CustomHTTPErrorHandler(logger *zap.Logger) echo.HTTPErrorHandler {
	return func(c *echo.Context, err error) {
		if resp, uErr := echo.UnwrapResponse(c.Response()); uErr == nil {
			if resp.Committed {
				return
			}
		}

		var appErr *errs.AppError
		var echoHTTPErr *echo.HTTPError

		res := Response{
			StatusCode: http.StatusInternalServerError,
			Message:    "Internal Server Error",
			ErrorKey:   "INTERNAL_SERVER_ERROR",
			TraceID:    c.Response().Header().Get(echo.HeaderXRequestID),
		}

		switch {
		case errors.As(err, &appErr):
			res.StatusCode = appErr.StatusCode
			res.Message = appErr.Message
			res.ErrorKey = appErr.Key

			if appErr.StatusCode >= http.StatusInternalServerError {
				logger.Error("AppError occurred", zap.String("error_key", appErr.Key), zap.String("message", appErr.Message), zap.String("trace_id", res.TraceID), zap.Error(appErr.RootErr))
			} else {
				logger.Info("AppError occurred", zap.String("error_key", appErr.Key), zap.String("message", appErr.Message), zap.String("trace_id", res.TraceID))
			}

		case errors.As(err, &echoHTTPErr):
			res.StatusCode = echoHTTPErr.Code
			res.Message = fmt.Sprintf("%v", echoHTTPErr.Message)
			res.ErrorKey = "ECHO_ERROR"

		default:
			logger.Error("Unknown Error", zap.String("error_key", err.Error()))
			res.Message = err.Error()
		}

		if c.Request().Method == http.MethodHead {
			err = c.NoContent(res.StatusCode)
		} else {
			err = c.JSON(res.StatusCode, res)
		}
	}
}
