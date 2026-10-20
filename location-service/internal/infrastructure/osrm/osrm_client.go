package osrm

import (
	"context"
	"echo-ride/services/location-service/internal/domain"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type osrmClient struct {
	baseURL    string
	httpClient *http.Client
	tracer     trace.Tracer
}

func NewOSRMClient(baseURL string) domain.RoutingService {
	return &osrmClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 2 * time.Second, // SECURITY: Set a reasonable timeout to prevent hanging
		},
		tracer: otel.Tracer("osrm-client"),
	}
}

type osrmTableResponse struct {
	Code      string      `json:"code"`
	Durations [][]float64 `json:"durations"`
	Distances [][]float64 `json:"distances"`
}

func (o *osrmClient) CalculateETAMatrix(ctx context.Context, originLat, originLng float64, destinations []domain.DriverLocation) ([]domain.DriverETA, error) {
	ctx, span := o.tracer.Start(ctx, "OSRM.CalculateETAMatrix")
	defer span.End()

	if len(destinations) == 0 {
		return nil, nil
	}

	coords := []string{fmt.Sprintf("%f,%f", originLng, originLat)}

	for _, dest := range destinations {
		coords = append(coords, fmt.Sprintf("%f,%f", dest.Lng, dest.Lat))
	}

	coordString := strings.Join(coords, ";")

	url := fmt.Sprintf("%s/table/v1/driving/%s?sources=0&annotations=duration,distance", o.baseURL, coordString)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create OSRM request: %w", err)
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send OSRM request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OSRM returned non-200 status: %d", resp.StatusCode)
	}

	var tableResp osrmTableResponse
	if err := json.NewDecoder(resp.Body).Decode(&tableResp); err != nil {
		return nil, fmt.Errorf("failed to decode OSRM response: %w", err)
	}

	if tableResp.Code != "Ok" {
		return nil, fmt.Errorf("OSRM response code not OK: %s", tableResp.Code)
	}

	var etas []domain.DriverETA
	for i, dest := range destinations {
		durationIndex := i + 1 // +1 because the first entry is the origin

		if len(tableResp.Durations) > 0 && len(tableResp.Durations[0]) > durationIndex {
			etas = append(etas, domain.DriverETA{
				DriverID: dest.DriverID,
				Lat:      dest.Lat,
				Lng:      dest.Lng,
				ETA:      tableResp.Durations[0][durationIndex],
				Distance: tableResp.Distances[0][durationIndex],
			})
		}
	}

	return etas, nil
}
