package audit

type httpAdapter struct {
	h *handler
}

func newHTTPAdapter(svc ServicePort) *httpAdapter {
	return &httpAdapter{
		h: newHandler(svc),
	}
}
