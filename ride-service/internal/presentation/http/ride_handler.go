package http

import (
	"echo-ride/pkg-library/errs"
	"echo-ride/pkg-library/middlewares"
	"echo-ride/pkg-library/response"
	"echo-ride/services/ride-service/internal/application"
	"echo-ride/services/ride-service/internal/domain"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type RideHandler struct {
	createRideUC   application.CreateRideUseCase
	updateRideUC   application.UpdateRideUseCase
	getRideUC      application.GetRideUseCase
	acceptRideUC   application.AcceptRideUseCase
	arriveRideUC   application.ArriveRideUseCase
	startTripUC    application.StartTripUseCase
	completeTripUC application.CompleteTripUseCase
	cancelRideUC   application.CanceledRideUC
	declineRideUC  application.DeclineRideUC
}

func NewRideHandler(
	e *echo.Echo,
	createRideUC application.CreateRideUseCase,
	updateRideUC application.UpdateRideUseCase,
	getRideUC application.GetRideUseCase,
	acceptRideUC application.AcceptRideUseCase,
	arriveRideUC application.ArriveRideUseCase,
	startTripUC application.StartTripUseCase,
	completeTripUC application.CompleteTripUseCase,
	cancelRideUC application.CanceledRideUC,
	declineRideUC application.DeclineRideUC,
) {
	handler := &RideHandler{
		createRideUC:   createRideUC,
		updateRideUC:   updateRideUC,
		getRideUC:      getRideUC,
		acceptRideUC:   acceptRideUC,
		arriveRideUC:   arriveRideUC,
		startTripUC:    startTripUC,
		completeTripUC: completeTripUC,
		cancelRideUC:   cancelRideUC,
		declineRideUC:  declineRideUC,
	}

	v1 := e.Group("/api/v1/rides")

	v1.Use(middlewares.RequireAuth())

	v1.POST("", handler.CreateRide, middlewares.RequireRole("RIDER"))
	v1.GET("", handler.ListRides)
	v1.GET("/:id", handler.GetByID)
	v1.PATCH("/:id/accept", handler.AcceptRide, middlewares.RequireRole("DRIVER"))
	v1.PATCH("/:id/status", handler.UpdateStatus)
	v1.PATCH("/:id/arrive", handler.ArriveRide, middlewares.RequireRole("DRIVER"))
	v1.PATCH("/:id/start", handler.StartTrip, middlewares.RequireRole("DRIVER"))
	v1.PATCH("/:id/complete", handler.CompleteTrip, middlewares.RequireRole("DRIVER"))
	v1.PATCH("/:id/cancel", handler.CancelRide, middlewares.RequireRole("RIDER", "DRIVER"))
	v1.PATCH("/:id/decline", handler.DeclineRide, middlewares.RequireRole("DRIVER"))
}

func (h *RideHandler) CreateRide(ctx *echo.Context) error {
	var req createRideRequest

	if err := ctx.Bind(&req); err != nil {
		return errs.ErrBadRequest.WithMessage("Failed to bind create ride").WithRootErr(err)
	}

	if err := ctx.Validate(&req); err != nil {
		return errs.ErrBadRequest.WithMessage("Validation failed").WithRootErr(err)
	}

	userIDStr, ok := ctx.Get("userId").(string)
	if !ok || userIDStr == "" {
		return errs.ErrUnauthorized.WithMessage("User context not found in context")
	}
	riderID, err := uuid.Parse(userIDStr)
	if err != nil {
		return errs.ErrBadRequest.WithMessage("Invalid X-User-Id").WithRootErr(err)
	}

	cmd := application.CreateRideCommand{
		RiderID:    riderID,
		PickupLat:  req.PickupLat,
		PickupLng:  req.PickupLon,
		DropoffLat: req.DropoffLat,
		DropoffLng: req.DropoffLon,
	}

	ride, err := h.createRideUC.Execute(ctx.Request().Context(), cmd)
	if err != nil {
		return errs.ErrInternal.WithMessage(err.Error())
	}

	resp := createRideResponse{
		RideID: ride.ID,
		Status: ride.Status,
		Price:  ride.Price,
	}

	return response.WriteSuccess(ctx, http.StatusCreated, resp, "Ride created successfully")
}

func (h *RideHandler) GetByID(ctx *echo.Context) error {
	idParam := ctx.Param("id")
	rideID, err := uuid.Parse(idParam)
	if err != nil {
		return errs.ErrBadRequest.WithMessage("Invalid ride ID").WithRootErr(err)
	}

	ride, err := h.getRideUC.GetByID(ctx.Request().Context(), rideID)
	if err != nil {
		return errs.ErrNotFound.WithMessage(err.Error())
	}

	return response.WriteSuccess(ctx, http.StatusOK, ride, "Ride retrieved successfully")
}

func (h *RideHandler) ListRides(ctx *echo.Context) error {
	filter := domain.RideFilter{
		Limit:  10,
		Offset: 0,
	}

	if s := ctx.QueryParam("status"); s != "" {
		filter.Status = &s
	}

	if v := ctx.QueryParam("rider_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return errs.ErrBadRequest.WithMessage("Invalid rider_id").WithRootErr(err)
		}
		filter.RiderID = &id
	}

	if v := ctx.QueryParam("driver_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return errs.ErrBadRequest.WithMessage("Invalid driver_id").WithRootErr(err)
		}
		filter.DriverID = &id
	}

	if v := ctx.QueryParam("limit"); v != "" {
		limit, err := strconv.Atoi(v)
		if err != nil {
			return errs.ErrBadRequest.WithMessage("Invalid limit").WithRootErr(err)
		}
		if limit > 100 {
			limit = 100
		}
		filter.Limit = int32(limit)
	}

	rides, err := h.getRideUC.ListRides(ctx.Request().Context(), filter)
	if err != nil {
		return errs.ErrInternal.WithMessage(err.Error())
	}

	return response.WriteSuccess(ctx, http.StatusOK, rides, "Rides listed successfully")
}

func (h *RideHandler) AcceptRide(ctx *echo.Context) error {
	rideIDStr := ctx.Param("id")
	rideID, err := uuid.Parse(rideIDStr)
	if err != nil {
		return errs.ErrBadRequest.WithMessage("Invalid ride ID").WithRootErr(err)
	}

	userIDStr, ok := ctx.Get("userId").(string)
	if !ok || userIDStr == "" {
		return errs.ErrUnauthorized.WithMessage("User context not found in context")
	}

	driverID, err := uuid.Parse(userIDStr)
	ride, err := h.acceptRideUC.Execute(ctx.Request().Context(), rideID, driverID)
	if err != nil {
		return err
	}

	return response.WriteSuccess(ctx, http.StatusOK, ride, "Ride accepted successfully")
}

func (h *RideHandler) UpdateStatus(ctx *echo.Context) error {
	rideIDStr := ctx.Param("id")
	rideID, err := uuid.Parse(rideIDStr)
	if err != nil {
		return errs.ErrInvalidInput.WithMessage("Invalid ride ID")
	}

	var req updateStatusRequest
	if err := ctx.Bind(&req); err != nil {
		return errs.ErrInvalidInput.WithRootErr(err)
	}
	if err := ctx.Validate(&req); err != nil {
		return errs.ErrInvalidInput.WithRootErr(err)
	}

	ride, err := h.updateRideUC.UpdateStatus(ctx.Request().Context(), rideID, req.Status)
	if err != nil {
		return err
	}

	return response.WriteSuccess(ctx, http.StatusOK, ride, "Ride status updated")
}

func (h *RideHandler) ArriveRide(ctx *echo.Context) error {
	rideID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		return errs.ErrBadRequest.WithMessage("Invalid ride ID").WithRootErr(err)
	}

	var req arriveRideRequest
	if err := ctx.Bind(&req); err != nil {
		return errs.ErrBadRequest.WithMessage("Invalid request body").WithRootErr(err)
	}
	if err := ctx.Validate(&req); err != nil {
		return errs.ErrBadRequest.WithMessage("Validation failed").WithRootErr(err)
	}

	driverID, err := driverIDFromContext(ctx)
	if err != nil {
		return err
	}

	if err := h.arriveRideUC.Execute(ctx.Request().Context(), application.ArriveRideCommand{
		RideID:    rideID,
		DriverID:  driverID,
		DriverLat: req.Lat,
		DriverLng: req.Lng,
	}); err != nil {
		return err
	}

	return response.WriteSuccess(ctx, http.StatusOK, map[string]string{
		"ride_id": rideID.String(),
		"status":  string(domain.EventTypeDriverArrived),
	}, "Driver arrival recorded")
}

func (h *RideHandler) StartTrip(ctx *echo.Context) error {
	rideID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		return errs.ErrBadRequest.WithMessage("Invalid ride ID").WithRootErr(err)
	}
	driverID, err := driverIDFromContext(ctx)
	if err != nil {
		return err
	}

	ride, err := h.startTripUC.Execute(ctx.Request().Context(), rideID, driverID)
	if err != nil {
		return err
	}

	return response.WriteSuccess(ctx, http.StatusOK, ride, "Trip started")
}

func (h *RideHandler) CompleteTrip(ctx *echo.Context) error {
	rideID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		return errs.ErrBadRequest.WithMessage("Invalid ride ID").WithRootErr(err)
	}
	driverID, err := driverIDFromContext(ctx)
	if err != nil {
		return err
	}

	ride, err := h.completeTripUC.Execute(ctx.Request().Context(), rideID, driverID)
	if err != nil {
		return err
	}

	return response.WriteSuccess(ctx, http.StatusOK, ride, "Trip completed")
}

func driverIDFromContext(ctx *echo.Context) (uuid.UUID, error) {
	userIDStr, ok := ctx.Get("userId").(string)
	if !ok || userIDStr == "" {
		return uuid.Nil, errs.ErrUnauthorized.WithMessage("Missing user context")
	}
	driverID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, errs.ErrUnauthorized.WithMessage("Invalid user id").WithRootErr(err)
	}
	return driverID, nil
}

func (h *RideHandler) CancelRide(ctx *echo.Context) error {
	rideID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		return errs.ErrBadRequest.WithMessage("Invalid ride ID").WithRootErr(err)
	}

	userIDStr, _ := ctx.Get("userId").(string)
	roleStr, _ := ctx.Get("userRole").(string)
	if userIDStr == "" || roleStr == "" {
		return errs.ErrUnauthorized.WithMessage("Missing user context")
	}

	actorID, err := uuid.Parse(userIDStr)
	if err != nil {
		return errs.ErrUnauthorized.WithMessage("Invalid user id").WithRootErr(err)
	}

	var cancelledBy application.CancelledBy
	switch roleStr {
	case "RIDER":
		cancelledBy = application.CancelledByRider
	case "DRIVER":
		cancelledBy = application.CancelledByDriver
	default:
		return errs.ErrForbidden.WithMessage("Role not allowed to cancel rides")
	}

	if err := h.cancelRideUC.Execute(ctx.Request().Context(), rideID, actorID, cancelledBy); err != nil {
		return err
	}

	return response.WriteSuccess(ctx, http.StatusOK, map[string]string{
		"ride_id":      rideID.String(),
		"status":       string(domain.RideStatusCancelled),
		"cancelled_by": string(cancelledBy),
	}, "Ride cancelled")
}

func (h *RideHandler) DeclineRide(ctx *echo.Context) error {
	rideID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		return errs.ErrBadRequest.WithMessage("Invalid ride ID").WithRootErr(err)
	}

	userIDStr, _ := ctx.Get("userId").(string)
	if userIDStr == "" {
		return errs.ErrUnauthorized.WithMessage("Missing user context")
	}

	driverID, err := uuid.Parse(userIDStr)
	if err != nil {
		return errs.ErrUnauthorized.WithMessage("Invalid user id").WithRootErr(err)
	}

	if err := h.declineRideUC.Execute(ctx.Request().Context(), rideID, driverID); err != nil {
		return err
	}

	return response.WriteSuccess(ctx, http.StatusOK, map[string]string{
		"ride_id": rideID.String(),
	}, "Ride offer declined")
}
