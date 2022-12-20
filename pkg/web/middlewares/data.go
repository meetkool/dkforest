package middlewares

type unauthorizedData struct {
	Message string
}

type captchaMiddlewareData struct {
	CaptchaID  string
	CaptchaImg string
	ErrCaptcha string
}
