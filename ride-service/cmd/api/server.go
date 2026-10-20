package main

import (
	"echo-ride/pkg-library/middlewares"
	"echo-ride/pkg-library/response"
	"echo-ride/services/ride-service/internal/application"
	"echo-ride/services/ride-service/internal/domain"
	"echo-ride/services/ride-service/internal/infrastructure/repository"
	rideHttp "echo-ride/services/ride-service/internal/presentation/http"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"go.uber.org/zap"
)

type customValidator struct {
	v *validator.Validate
}

func (cv *customValidator) Validate(i interface{}) error {
	return cv.v.Struct(i)
}

// appUseCases groups all use-cases for ride-service so we can wire both the
// HTTP server and the Kafka consumer from main without instantiating twice.
type appUseCases struct {
	repo         domain.RideRepository
	createRide   application.CreateRideUseCase
	updateRide   application.UpdateRideUseCase
	getRide      application.GetRideUseCase
	acceptRide   application.AcceptRideUseCase
	arriveRide   application.ArriveRideUseCase
	startTrip    application.StartTripUseCase
	completeTrip application.CompleteTripUseCase
	cancelRide   application.CanceledRideUC
	declineRide  application.DeclineRideUC
}

func buildUseCases(dbPool *pgxpool.Pool, log *zap.Logger) *appUseCases {
	repo := repository.NewRideRepository(dbPool)
	return &appUseCases{
		repo:         repo,
		createRide:   application.NewCreateRideUseCase(repo),
		updateRide:   application.NewUdpateRideUseCase(repo, log),
		getRide:      application.NewGetRideUseCase(repo),
		acceptRide:   application.NewAcceptRideUseCase(repo, log),
		arriveRide:   application.NewArriveRideUseCase(repo, log),
		startTrip:    application.NewStartTripUseCase(repo, log),
		completeTrip: application.NewCompleteTripUseCase(repo, log),
		cancelRide:   application.NewCancelRideUseCase(repo, log),
		declineRide:  application.NewDeclineRideUseCase(repo, log),
	}
}

func newServer(uc *appUseCases, log *zap.Logger) *echo.Echo {
	e := echo.New()
	e.Validator = &customValidator{v: validator.New()}
	e.Use(middleware.Recover())
	e.Use(middlewares.OTelMiddleware("ride-service"))
	e.HTTPErrorHandler = response.CustomHTTPErrorHandler(log)

	e.GET("/health", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok", "service": "ride-service"})
	})

	rideHttp.NewRideHandler(e, uc.createRide, uc.updateRide, uc.getRide, uc.acceptRide, uc.arriveRide, uc.startTrip, uc.completeTrip, uc.cancelRide, uc.declineRide)

	return e
}
