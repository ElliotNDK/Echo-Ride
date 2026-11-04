module echo-ride/services/payment-service

go 1.26.0

require github.com/labstack/echo/v5 v5.4.0

require (
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/time v0.15.0 // indirect
)

replace echo-ride/pkg-library => ../pkg-library
