package errs

var (
	ErrNotFound             = NewAppError(404, "Resource not found", "not_found")
	ErrUnauthorized         = NewAppError(401, "Unauthorized", "unauthorized")
	ErrBadRequest           = NewAppError(400, "Bad request", "bad_request")
	ErrConflict             = NewAppError(409, "Conflict", "conflict")
	ErrForbidden            = NewAppError(403, "Forbidden", "forbidden")
	ErrInvalidInput         = NewAppError(422, "Invalid input", "invalid_input")
	ErrInternal             = NewAppError(500, "Internal server error", "internal_error")
	ErrInvalidArgument      = NewAppError(400, "Invalid argument", "invalid_argument")
	ErrServiceConnectFailed = NewAppError(500, "Failed to connect to service", "service_connect_failed")
	ErrServiceCallFailed    = NewAppError(500, "Failed to call service", "service_call_failed")

	// Ride service specific errors
	ErrSamePickupAndDropoff = NewAppError(400, "Pickup and dropoff locations cannot be the same", "same_pickup_and_dropoff")

	// Location service specific errors
	ErrInvalidDriverID        = NewAppError(400, "Invalid driver ID", "invalid_driver_id")
	ErrWebsocketUpgradeFailed = NewAppError(500, "Failed to upgrade to websocket", "websocket_upgrade_failed")
)
