package response

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type PaginationBuilder struct {
	BaseURL     string
	Route       string
	QueryParams map[string]string
}

func NewPaginationBuilder(baseURL, route string) *PaginationBuilder {
	return &PaginationBuilder{
		BaseURL:     strings.TrimSuffix(baseURL, "/"),
		Route:       strings.TrimPrefix(route, "/"),
		QueryParams: make(map[string]string),
	}
}

func (pb *PaginationBuilder) WithQueryParams(params map[string]string) *PaginationBuilder {
	for k, v := range params {
		pb.QueryParams[k] = v
	}
	return pb
}

func (pb *PaginationBuilder) BuildURL(page int) string {
	baseURL := fmt.Sprintf("%s/%s", pb.BaseURL, pb.Route)
	query := url.Values{}

	for k, v := range pb.QueryParams {
		query.Set(k, v)
	}
	query.Set("page", strconv.Itoa(page))

	if encoded := query.Encode(); encoded != "" {
		return fmt.Sprintf("%s?%s", baseURL, encoded)
	}
	return baseURL
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
		from = 0
		to = 0
	}

	builder := NewPaginationBuilder(baseURL, route).WithQueryParams(queryParams)

	links := PaginationLinks{
		Self:  builder.BuildURL(currentPage),
		First: builder.BuildURL(1),
		Last:  builder.BuildURL(totalPages),
	}

	if currentPage < totalPages {
		links.Next = builder.BuildURL(currentPage + 1)
	}

	if currentPage > 1 {
		links.Prev = builder.BuildURL(currentPage - 1)
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
			Path:        fmt.Sprintf("%s/%s", baseURL, route),
		},
	}
}

func ParsePaginationRequest(c *fiber.Ctx) PaginationRequest {
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("page_size", 20)
	sortBy := c.Query("sort_by", "")
	sortOrder := c.Query("sort_order", "asc")

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
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}
}

func GetBaseURL(c *fiber.Ctx) string {
	scheme := "http"
	if c.Protocol() == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, c.Hostname())
}
