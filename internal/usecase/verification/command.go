package verification

type ResendVerificationCmd struct {
	Email string
}

type VerifyEmailCmd struct {
	Token     string
	IP        string
	UserAgent string
}

type ForgotPasswordCmd struct {
	Email string
}

type ResetPasswordCmd struct {
	Token     string
	Password  string
	IP        string
	UserAgent string
}
