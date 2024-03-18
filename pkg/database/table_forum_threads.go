package database

import (
	"database/sql"
	"fmt"
	"html"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/jmoiron/sqlx"
	"github.com/russross/blackfriday/v2"
)

type ForumCategory struct {
	ID   int64
	Idx  int6
