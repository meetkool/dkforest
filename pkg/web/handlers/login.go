package handlers

import (
	"bytes"
	"crypto/bcrypt"
	"fmt"
	"github.com/asaskevich/govalidator"
	"github.com/labstack/echo"
	"github.com/pquerna/otp/totp"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"strings"
	"time"
)

const (
	max2faAttempts = 4
)

type PartialAuthItem struct {
	UserID          database.UserID
	Step            PartialAuthStep
	SessionDuration time.Duration
	Attempt         int
}

type PartialAuthStep string

const (
	TwoFactorStep         PartialAuthStep = "2fa"
	PgpSignStep           PartialAuthStep = "pgp_sign_2fa"
	PgpStep               PartialAuthStep = "pgp_2fa"
)

type PartialRecoveryItem struct {
	UserID database.UserID
	Step   RecoveryStep
}

type RecoveryStep int6
