package auth

type LoginCmd struct {
	Email     string
	Password  string
	IP        string
	UserAgent string
}

type RefreshCmd struct {
	RefreshToken string
}

type LogoutCmd struct {
	RefreshToken string
	IP           string
	UserAgent    string
}

type ChangePasswordCmd struct {
	UserID          uint
	CurrentPassword string
	NewPassword     string
	IP              string
	UserAgent       string
}
