package handlers

import (
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/labstack/echo"
	"gorm.io/gorm"
)

type Paginator struct {
	ResultsPerPage int64
	WantedPageQP   string
}

func NewPaginator() *Paginator {
	return &Paginator{
		ResultsPerPage: 50,
		WantedPageQP:   "p",
	}
}

func (p *Paginator) SetResultsPerPage(v int64) *Paginator {
	p.ResultsPerPage = v
	return p
}

func (p *Paginator) SetWantedPageQP(v string) *Paginator {
	p.WantedPageQP = v
	return p
}

func (p *Paginator) Paginate(c echo.Context, query *gorm.DB) (int64, int64, int64, *gorm.DB) {
	wantedPageStr := c.QueryParam(p.WantedPageQP)
	wantedPage, err := strconv.ParseInt(wantedPageStr, 10, 64)
	if err != nil || wantedPage < 1 {
		wantedPage = 1
	}

	var count int6
