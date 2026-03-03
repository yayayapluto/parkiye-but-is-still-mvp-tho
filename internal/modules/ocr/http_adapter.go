package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// httpAdapter handles all HTTP communication with the Python OCR service.
type httpAdapter struct {
	client  *http.Client
	baseURL string
}

func newHTTPAdapter(baseURL string, timeout time.Duration) *httpAdapter {
	return &httpAdapter{
		client:  &http.Client{Timeout: timeout},
		baseURL: baseURL,
	}
}

// ping checks whether the Python OCR service is reachable via GET /health.
func (a *httpAdapter) ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("build health request: %w", err)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}
	return nil
}

// callDetectPlate sends imagePath to POST /detect-plate and returns the result.
// Returns an error for non-2xx responses or network failures.
func (a *httpAdapter) callDetectPlate(ctx context.Context, imagePath string) (*detectPlateResponse, error) {
	body, err := json.Marshal(detectPlateRequest{ImagePath: imagePath})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/detect-plate", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http call failed: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var result detectPlateResponse
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("decode success response: %w", err)
		}
		return &result, nil
	}

	// Non-2xx: decode error detail from FastAPI
	var errResp detectPlateErrorResponse
	if err := json.Unmarshal(respBody, &errResp); err != nil {
		return nil, fmt.Errorf("ocr api error (status %d): %s", resp.StatusCode, string(respBody))
	}
	return nil, fmt.Errorf("ocr api error (status %d): %s", resp.StatusCode, errResp.Detail)
}
