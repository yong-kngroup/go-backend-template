package cms

import "errors"

var (
	ErrInvalidInput      = errors.New("invalid CMS input")
	ErrCategoryNotFound  = errors.New("category not found")
	ErrArticleNotFound   = errors.New("article not found")
	ErrTranslationAbsent = errors.New("content translation not found")
	ErrCategoryCycle     = errors.New("category move would create a cycle")
	ErrLocaleNotFound    = errors.New("locale not found")
)
