package cms

import "errors"

var (
	ErrInvalidInput      = errors.New("invalid CMS input")
	ErrCategoryNotFound  = errors.New("category not found")
	ErrArticleNotFound   = errors.New("article not found")
	ErrTranslationAbsent = errors.New("content translation not found")
	ErrCategoryCycle     = errors.New("category move would create a cycle")
	ErrLocaleNotFound    = errors.New("locale not found")
	ErrLocaleDefault     = errors.New("default locale cannot be disabled")
	ErrLastEnabledLocale = errors.New("at least one locale must remain enabled")
	ErrArticleDeleted    = errors.New("article already deleted")
	ErrArticleActive     = errors.New("article is not deleted")
	ErrSlugReserved      = errors.New("slug is reserved by a redirect")
	ErrRedirectNotFound  = errors.New("redirect not found")
	ErrTagNotFound       = errors.New("tag not found")
)
