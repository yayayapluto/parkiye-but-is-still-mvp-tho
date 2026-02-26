package response

type Meta struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type SuccessResponse struct {
	Success bool   `json:"success"`
	Meta    Meta   `json:"meta"`
	Data    any    `json:"data,omitempty"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Meta    Meta   `json:"meta"`
	Data    any    `json:"data,omitempty"`
}

type PaginationRequest struct {
	Page      int               `json:"page" query:"page"`
	PageSize  int               `json:"page_size" query:"page_size"`
	SortBy    string            `json:"sort_by,omitempty" query:"sort_by"`
	SortOrder string            `json:"sort_order,omitempty" query:"sort_order"`
	Filters   map[string]string `json:"filters,omitempty"`
}

type PaginationLinks struct {
	First string `json:"first"`
	Last  string `json:"last"`
	Next  string `json:"next,omitempty"`
	Prev  string `json:"prev,omitempty"`
	Self  string `json:"self"`
}

type PaginationMeta struct {
	CurrentPage int    `json:"current_page"`
	From        int    `json:"from"`
	LastPage    int    `json:"last_page"`
	PageSize    int    `json:"per_page"`
	To          int    `json:"to"`
	Total       int64  `json:"total"`
	Path        string `json:"path"`
}

type Pagination struct {
	Links PaginationLinks `json:"links"`
	Meta  PaginationMeta  `json:"meta"`
}

type PaginatedResponse struct {
	Success    bool        `json:"success"`
	Status     int         `json:"status"`
	Message    string      `json:"message"`
	Data       any         `json:"data"`
	Pagination Pagination  `json:"pagination"`
}
