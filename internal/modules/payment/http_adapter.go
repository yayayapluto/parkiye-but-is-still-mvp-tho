package payment

import "parkieee/pkg/validator"

type httpAdapter struct{ h *handler }

func newHTTPAdapter(svc ServicePort, v *validator.Validator) *httpAdapter {
	return &httpAdapter{h: newHandler(svc, v)}
}
