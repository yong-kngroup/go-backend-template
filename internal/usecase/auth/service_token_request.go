package auth

type IssueServiceTokenCmd struct {
	ClientID     string
	ClientSecret string
	IP           string
	UserAgent    string
}
