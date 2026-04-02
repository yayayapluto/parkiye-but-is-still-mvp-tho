package payment

import (
	zoneDomain "parkieee/internal/modules/zone"
	"parkieee/pkg/validator"
)

type httpAdapter struct{ h *handler }

func newHTTPAdapter(svc ServicePort, v *validator.Validator, assignmentRepo zoneDomain.GateCashierAssignmentRepositoryPort) *httpAdapter {
	return &httpAdapter{h: newHandler(svc, v, assignmentRepo)}
}
