package cms

import (
	"context"
	"errors"
	domainCMS "github.com/freeDog-wy/go-backend-template/internal/domain/cms"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	"strings"
	"time"
)

type Service struct {
	tx   shared.TxManager
	repo domainCMS.Repository
	now  func() time.Time
}

func New(tx shared.TxManager, repo domainCMS.Repository) *Service {
	return &Service{tx: tx, repo: repo, now: time.Now}
}

func (s *Service) CreateCategory(ctx context.Context, cmd CreateCategoryCmd) (*CategoryResult, error) {
	if err := validNameSlug(cmd.Name, cmd.Slug); err != nil {
		return nil, err
	}
	if err := s.requireLocale(ctx, cmd.Locale); err != nil {
		return nil, err
	}
	if cmd.ParentID != nil {
		if _, err := s.repo.FindCategory(ctx, *cmd.ParentID); err != nil {
			return nil, mapCategory(err)
		}
	}
	c := &domainCMS.Category{ParentID: cmd.ParentID, SortOrder: cmd.SortOrder, Enabled: true}
	tr := &domainCMS.CategoryTranslation{Locale: cmd.Locale, Name: strings.TrimSpace(cmd.Name), Slug: strings.TrimSpace(cmd.Slug), Description: cmd.Description, SEOTitle: cmd.SEOTitle, SEODescription: cmd.SEODescription}
	if err := s.repo.CreateCategory(ctx, c, tr); err != nil {
		return nil, err
	}
	return &CategoryResult{ID: c.ID, ParentID: c.ParentID, SortOrder: c.SortOrder, Locale: tr.Locale, Name: tr.Name, Slug: tr.Slug}, nil
}
func (s *Service) MoveCategory(ctx context.Context, cmd MoveCategoryCmd) error {
	if cmd.CategoryID == 0 {
		return domainCMS.ErrInvalidInput
	}
	if _, err := s.repo.FindCategory(ctx, cmd.CategoryID); err != nil {
		return mapCategory(err)
	}
	if cmd.ParentID != nil {
		if *cmd.ParentID == cmd.CategoryID {
			return domainCMS.ErrCategoryCycle
		}
		if _, err := s.repo.FindCategory(ctx, *cmd.ParentID); err != nil {
			return mapCategory(err)
		}
		descendant, err := s.repo.IsCategoryDescendant(ctx, cmd.CategoryID, *cmd.ParentID)
		if err != nil {
			return err
		}
		if descendant {
			return domainCMS.ErrCategoryCycle
		}
	}
	return s.repo.MoveCategory(ctx, cmd.CategoryID, cmd.ParentID, cmd.SortOrder)
}
func (s *Service) CreateArticle(ctx context.Context, cmd CreateArticleCmd) (*ArticleResult, error) {
	if cmd.AuthorUserID == 0 {
		return nil, domainCMS.ErrInvalidInput
	}
	if err := s.requireLocale(ctx, cmd.Locale); err != nil {
		return nil, err
	}
	if err := validArticle(cmd.Title, cmd.Slug, cmd.ContentFormat); err != nil {
		return nil, err
	}
	a := &domainCMS.Article{AuthorUserID: cmd.AuthorUserID}
	tr := translationFromCreate(0, cmd)
	if err := s.tx.Do(ctx, func(ctx context.Context) error { return s.repo.CreateArticle(ctx, a, tr) }); err != nil {
		return nil, err
	}
	return articleResult(a.ID, tr), nil
}
func (s *Service) CreateTranslation(ctx context.Context, cmd CreateTranslationCmd) (*ArticleResult, error) {
	if cmd.ArticleID == 0 {
		return nil, domainCMS.ErrInvalidInput
	}
	if err := s.requireLocale(ctx, cmd.Locale); err != nil {
		return nil, err
	}
	if err := validArticle(cmd.Title, cmd.Slug, cmd.ContentFormat); err != nil {
		return nil, err
	}
	tr := translationFromCreate(cmd.ArticleID, CreateArticleCmd{Locale: cmd.Locale, Title: cmd.Title, Slug: cmd.Slug, Summary: cmd.Summary, Content: cmd.Content, ContentFormat: cmd.ContentFormat, SEOTitle: cmd.SEOTitle, SEODescription: cmd.SEODescription, CanonicalURL: cmd.CanonicalURL})
	if err := s.repo.CreateArticleTranslation(ctx, tr); err != nil {
		return nil, err
	}
	return articleResult(cmd.ArticleID, tr), nil
}
func (s *Service) UpdateTranslation(ctx context.Context, cmd UpdateTranslationCmd) (*ArticleResult, error) {
	if err := validArticle(cmd.Title, cmd.Slug, cmd.ContentFormat); err != nil {
		return nil, err
	}
	tr, err := s.translation(ctx, cmd.ArticleID, cmd.Locale)
	if err != nil {
		return nil, err
	}
	tr.Title, tr.Slug, tr.Summary, tr.Content, tr.ContentFormat, tr.SEOTitle, tr.SEODescription, tr.CanonicalURL = cmd.Title, cmd.Slug, cmd.Summary, cmd.Content, cmd.ContentFormat, cmd.SEOTitle, cmd.SEODescription, cmd.CanonicalURL
	if err := s.repo.SaveArticleTranslation(ctx, tr); err != nil {
		return nil, err
	}
	return articleResult(cmd.ArticleID, tr), nil
}
func (s *Service) PublishTranslation(ctx context.Context, cmd PublishTranslationCmd) (*ArticleResult, error) {
	tr, err := s.translation(ctx, cmd.ArticleID, cmd.Locale)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	tr.Status, tr.PublishedAt = domainCMS.TranslationPublished, &now
	if err := s.repo.SaveArticleTranslation(ctx, tr); err != nil {
		return nil, err
	}
	return articleResult(cmd.ArticleID, tr), nil
}
func (s *Service) ArchiveTranslation(ctx context.Context, cmd ArchiveTranslationCmd) (*ArticleResult, error) {
	tr, err := s.translation(ctx, cmd.ArticleID, cmd.Locale)
	if err != nil {
		return nil, err
	}
	tr.Status = domainCMS.TranslationArchived
	if err := s.repo.SaveArticleTranslation(ctx, tr); err != nil {
		return nil, err
	}
	return articleResult(cmd.ArticleID, tr), nil
}
func (s *Service) ListCategories(ctx context.Context, cmd ListCategoriesCmd) ([]*CategoryTreeResult, error) {
	if err := s.requireLocale(ctx, cmd.Locale); err != nil {
		return nil, err
	}
	items, err := s.repo.ListCategoryTreeItems(ctx, cmd.Locale)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint]*CategoryTreeResult, len(items))
	for _, item := range items {
		byID[item.ID] = &CategoryTreeResult{ID: item.ID, ParentID: item.ParentID, SortOrder: item.SortOrder, Name: item.Name, Slug: item.Slug, Description: item.Description, Children: make([]*CategoryTreeResult, 0)}
	}
	roots := make([]*CategoryTreeResult, 0)
	for _, item := range items {
		node := byID[item.ID]
		if item.ParentID != nil {
			if parent, ok := byID[*item.ParentID]; ok {
				parent.Children = append(parent.Children, node)
				continue
			}
		}
		roots = append(roots, node)
	}
	return roots, nil
}
func (s *Service) ReplaceArticleCategories(ctx context.Context, cmd ReplaceArticleCategoriesCmd) error {
	if cmd.ArticleID == 0 {
		return domainCMS.ErrInvalidInput
	}
	if _, err := s.repo.FindArticle(ctx, cmd.ArticleID); err != nil {
		return mapArticle(err)
	}
	ids := make([]uint, 0, len(cmd.CategoryIDs))
	seen := make(map[uint]struct{}, len(cmd.CategoryIDs))
	for _, id := range cmd.CategoryIDs {
		if id == 0 {
			return domainCMS.ErrInvalidInput
		}
		if _, ok := seen[id]; ok {
			return domainCMS.ErrInvalidInput
		}
		seen[id] = struct{}{}
		if _, err := s.repo.FindCategory(ctx, id); err != nil {
			return mapCategory(err)
		}
		ids = append(ids, id)
	}
	if cmd.PrimaryCategoryID != nil {
		if _, ok := seen[*cmd.PrimaryCategoryID]; !ok {
			return domainCMS.ErrInvalidInput
		}
	}
	return s.tx.Do(ctx, func(ctx context.Context) error {
		return s.repo.ReplaceArticleCategories(ctx, cmd.ArticleID, ids, cmd.PrimaryCategoryID)
	})
}
func (s *Service) ListArticles(ctx context.Context, cmd ListArticlesCmd) ([]*ArticleResult, shared.PageResult, error) {
	if err := s.requireLocale(ctx, cmd.Locale); err != nil {
		return nil, shared.PageResult{}, err
	}
	page := shared.NewPageQuery(cmd.Page.Page, cmd.Page.PerPage)
	items, total, err := s.repo.ListArticleTranslations(ctx, cmd.Locale, page)
	if err != nil {
		return nil, shared.PageResult{}, err
	}
	results := make([]*ArticleResult, 0, len(items))
	for _, item := range items {
		results = append(results, articleResult(item.Article.ID, &item.ArticleTranslation))
	}
	return results, shared.PageResult{Page: page.Page, PerPage: page.PerPage, Total: total}, nil
}
func (s *Service) GetPublishedArticle(ctx context.Context, locale, slug string) (*PublicArticleResult, error) {
	a, err := s.repo.FindPublicArticle(ctx, locale, slug)
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return nil, domainCMS.ErrTranslationAbsent
		}
		return nil, err
	}
	return &PublicArticleResult{ID: a.Article.ID, Locale: a.Locale, Title: a.Title, Slug: a.Slug, Summary: a.Summary, Content: a.Content, ContentFormat: a.ContentFormat, PublishedAt: a.PublishedAt, SEOTitle: a.SEOTitle, SEODescription: a.SEODescription, CanonicalURL: a.CanonicalURL, UpdatedAt: a.ArticleTranslation.UpdatedAt}, nil
}
func (s *Service) translation(ctx context.Context, id uint, locale string) (*domainCMS.ArticleTranslation, error) {
	if id == 0 || strings.TrimSpace(locale) == "" {
		return nil, domainCMS.ErrInvalidInput
	}
	tr, err := s.repo.FindArticleTranslation(ctx, id, locale)
	if errors.Is(err, shared.ErrNotFound) {
		return nil, domainCMS.ErrTranslationAbsent
	}
	return tr, err
}
func (s *Service) requireLocale(ctx context.Context, locale string) error {
	ok, err := s.repo.LocaleEnabled(ctx, strings.TrimSpace(locale))
	if err != nil {
		return err
	}
	if !ok {
		return domainCMS.ErrLocaleNotFound
	}
	return nil
}
func validNameSlug(name, slug string) error {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(slug) == "" {
		return domainCMS.ErrInvalidInput
	}
	return nil
}
func validArticle(title, slug, format string) error {
	if err := validNameSlug(title, slug); err != nil {
		return err
	}
	if format != "" && format != "markdown" && format != "html" {
		return domainCMS.ErrInvalidInput
	}
	return nil
}
func mapCategory(err error) error {
	if errors.Is(err, shared.ErrNotFound) {
		return domainCMS.ErrCategoryNotFound
	}
	return err
}
func mapArticle(err error) error {
	if errors.Is(err, shared.ErrNotFound) {
		return domainCMS.ErrArticleNotFound
	}
	return err
}
func translationFromCreate(articleID uint, cmd CreateArticleCmd) *domainCMS.ArticleTranslation {
	format := cmd.ContentFormat
	if format == "" {
		format = "markdown"
	}
	return &domainCMS.ArticleTranslation{ArticleID: articleID, Locale: cmd.Locale, Title: cmd.Title, Slug: cmd.Slug, Summary: cmd.Summary, Content: cmd.Content, ContentFormat: format, Status: domainCMS.TranslationDraft, SEOTitle: cmd.SEOTitle, SEODescription: cmd.SEODescription, CanonicalURL: cmd.CanonicalURL}
}
func articleResult(id uint, tr *domainCMS.ArticleTranslation) *ArticleResult {
	return &ArticleResult{ID: id, Locale: tr.Locale, Title: tr.Title, Slug: tr.Slug, Status: string(tr.Status), PublishedAt: tr.PublishedAt}
}
