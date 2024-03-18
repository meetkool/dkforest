package database

import (
	"crypto/sha512"
	"dkforest/pkg/config"
	"dkforest/pkg/utils"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

// Gist struct represents a gist in the database
type Gist struct {
	ID        int6
