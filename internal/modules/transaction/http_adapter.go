package transaction

import (
	"parkieee/pkg/config"
	"parkieee/pkg/validator"
)

type httpAdapter struct{ h *handler }

func newHTTPAdapter(svc ServicePort, v *validator.Validator, s3cfg config.S3Config) *httpAdapter {
	return &httpAdapter{h: newHandler(svc, v, s3cfg)}
}
