package transaction

import "parkieee/pkg/validator"

type httpAdapter struct{ h *handler }

func newHTTPAdapter(svc ServicePort, v *validator.Validator, storageDir, ocrPrefix string) *httpAdapter {
	return &httpAdapter{h: newHandler(svc, v, storageDir, ocrPrefix)}
}
