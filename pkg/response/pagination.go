package response

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type PaginationRequest struct {
	Page      int               `json:"page"                 query:"page"`
	PageSize  int               `json:"page_size"            query:"page_size"`
	SortBy    string            `json:"sort_by,omitempty"    query:"sort_by"`
	SortOrder string            `json:"sort_order,omitempty" query:"sort_order"`
	Filters   map[string]string `json:"filters,omitempty"`
}

type PaginationLinks struct {
	Self  string `json:"self"`
	First string `json:"first"`
	Last  string `json:"last"`
	Next  string `json:"next,omitempty"`
	Prev  string `json:"prev,omitempty"`
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
	Success    bool       `json:"success"`
	Meta       Meta       `json:"meta"`
	Data       any        `json:"data"`
	Pagination Pagination `json:"pagination"`
}

func Paginated(c *fiber.Ctx, message string, data any, pagination Pagination) error {
	return c.Status(200).JSON(PaginatedResponse{
		Success:    true,
		Meta:       Meta{Code: "OK", Message: message},
		Data:       data,
		Pagination: pagination,
	})
}

func GeneratePagination(
	baseURL string,
	route string,
	currentPage int,
	pageSize int,
	totalItems int64,
	queryParams map[string]string,
) Pagination {
	if currentPage < 1 {
		currentPage = 1
	}

	totalPages := int(math.Ceil(float64(totalItems) / float64(pageSize)))
	if totalPages < 1 {
		totalPages = 1
	}

	from := (currentPage-1)*pageSize + 1
	to := currentPage * pageSize
	if int64(to) > totalItems {
		to = int(totalItems)
	}
	if totalItems == 0 {
		from, to = 0, 0
	}

	builder := newPaginationBuilder(baseURL, route).withQueryParams(queryParams)

	links := PaginationLinks{
		Self:  builder.buildURL(currentPage),
		First: builder.buildURL(1),
		Last:  builder.buildURL(totalPages),
	}
	if currentPage < totalPages {
		links.Next = builder.buildURL(currentPage + 1)
	}
	if currentPage > 1 {
		links.Prev = builder.buildURL(currentPage - 1)
	}

	return Pagination{
		Links: links,
		Meta: PaginationMeta{
			CurrentPage: currentPage,
			From:        from,
			LastPage:    totalPages,
			PageSize:    pageSize,
			To:          to,
			Total:       totalItems,
			Path:        fmt.Sprintf("%s/%s", strings.TrimSuffix(baseURL, "/"), strings.TrimPrefix(route, "/")),
		},
	}
}

func ParsePaginationRequest(c *fiber.Ctx) PaginationRequest {
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("page_size", 20)

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	} else if pageSize > 100 {
		pageSize = 100
	}

	return PaginationRequest{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order", "asc"),
	}
}

func GetBaseURL(c *fiber.Ctx) string {
	return fmt.Sprintf("%s://%s", c.Protocol(), c.Hostname())
}

type paginationBuilder struct {
	base  string
	route string
	query map[string]string
}

func newPaginationBuilder(base, route string) *paginationBuilder {
	return &paginationBuilder{
		base:  strings.TrimSuffix(base, "/"),
		route: strings.TrimPrefix(route, "/"),
		query: make(map[string]string),
	}
}

func (pb *paginationBuilder) withQueryParams(params map[string]string) *paginationBuilder {
	for k, v := range params {
		pb.query[k] = v
	}
	return pb
}

func (pb *paginationBuilder) buildURL(page int) string {
	q := url.Values{}
	for k, v := range pb.query {
		q.Set(k, v)
	}
	q.Set("page", strconv.Itoa(page))
	return fmt.Sprintf("%s/%s?%s", pb.base, pb.route, q.Encode())
}
