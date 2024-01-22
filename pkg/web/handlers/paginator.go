package handlers

import (
	"dkforest/pkg/utils"
	"github.com/labstack/echo"
	"gorm.io/gorm"
	"math"
)

type Paginator struct {
	resultsPerPage       int64
	wantedPageQueryParam string
}

func NewPaginator() *Paginator {
	return &Paginator{
		wantedPageQueryParam: "p",
		resultsPerPage:       50,
	}
}

func (p *Paginator) SetResultPerPage(v int64) *Paginator {
	p.resultsPerPage = v
	return p
}

func (p *Paginator) SetWantedPageQueryParam(v string) *Paginator {
	p.wantedPageQueryParam = v
	return p
}

func (p *Paginator) Paginate(c echo.Context, query *gorm.DB) (int64, int64, int64, *gorm.DB) {
	wantedPage := utils.DoParseInt64(c.QueryParam(p.wantedPageQueryParam))
	var count int64
	query.Session(&gorm.Session{}).Count(&count)
	resultsPerPage := p.resultsPerPage
	page, maxPage := paginate(resultsPerPage, wantedPage, count)
	query = query.Offset(int((page - 1) * resultsPerPage)).Limit(int(resultsPerPage))
	return page, maxPage, count, query
}

func paginate(resultsPerPage, wantedPage, size int64) (page int64, maxPage int64) {
	page = wantedPage
	if page <= 1 {
		page = 1
	}
	maxPage = int64(math.Ceil(float64(size) / float64(resultsPerPage)))
	if maxPage <= 1 {
		maxPage = 1
	}
	if page > maxPage {
		page = maxPage
	}
	return
}
