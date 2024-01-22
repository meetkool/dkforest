package middlewares

type unauthorizedData struct {
	Message string
}

type captchaMiddlewareData struct {
	CaptchaDescription string
	CaptchaID          string
	CaptchaImg         string
	CaptchaAnswerImg   string
	ErrCaptcha         string
}
