package cms

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	domainAudit "github.com/freeDog-wy/go-backend-template/internal/domain/audit"
	domainCMS "github.com/freeDog-wy/go-backend-template/internal/domain/cms"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	"strings"
	"time"
)

type Service struct {
	tx       shared.TxManager
	repo     domainCMS.Repository
	now      func() time.Time
	eventBus shared.EventBus
}

func New(tx shared.TxManager, repo domainCMS.Repository, eventBuses ...shared.EventBus) *Service {
	service := &Service{tx: tx, repo: repo, now: time.Now}
	if len(eventBuses) > 0 {
		service.eventBus = eventBuses[0]
	}
	return service
}

func (s *Service) ListLocales(ctx context.Context) ([]*LocaleResult, error) {
	locales, err := s.repo.ListLocales(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*LocaleResult, 0, len(locales))
	for _, locale := range locales {
		result = append(result, localeResult(locale))
	}
	return result, nil
}
func (s *Service) CreateTag(ctx context.Context, cmd CreateTagCmd) (*TagResult, error) {
	if err := validNameSlug(cmd.Name, cmd.Slug); err != nil {
		return nil, err
	}
	if err := s.requireLocale(ctx, cmd.Locale); err != nil {
		return nil, err
	}
	tag := &domainCMS.Tag{}
	tr := &domainCMS.TagTranslation{Locale: strings.TrimSpace(cmd.Locale), Name: strings.TrimSpace(cmd.Name), Slug: strings.TrimSpace(cmd.Slug)}
	if err := s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.ensureSlugAvailable(ctx, tr.Locale, tagPath(tr.Locale, tr.Slug)); err != nil {
			return err
		}
		if err := s.repo.CreateTag(ctx, tag, tr); err != nil {
			return err
		}
		return s.publishAudit(ctx, cmd.ActorUserID, "tag", tag.ID, domainAudit.ActionCMSTagCreated, cmd.IP, cmd.UserAgent, map[string]any{"locale": tr.Locale, "slug": tr.Slug})
	}); err != nil {
		return nil, err
	}
	return tagResult(tag.ID, tr), nil
}
func (s *Service) ListTags(ctx context.Context, cmd ListTagsCmd) ([]*TagResult, shared.PageResult, error) {
	if err := s.requireExistingLocale(ctx, cmd.Locale); err != nil {
		return nil, shared.PageResult{}, err
	}
	page := shared.NewPageQuery(cmd.Page.Page, cmd.Page.PerPage)
	items, total, err := s.repo.ListTags(ctx, cmd.Locale, page)
	if err != nil {
		return nil, shared.PageResult{}, err
	}
	out := make([]*TagResult, 0, len(items))
	for _, v := range items {
		out = append(out, tagResult(v.ID, &v.TagTranslation))
	}
	return out, shared.PageResult{Page: page.Page, PerPage: page.PerPage, Total: total}, nil
}
func (s *Service) ReplaceArticleTags(ctx context.Context, cmd ReplaceArticleTagsCmd) error {
	if cmd.ArticleID == 0 {
		return domainCMS.ErrInvalidInput
	}
	if _, err := s.repo.FindArticle(ctx, cmd.ArticleID); err != nil {
		return mapArticle(err)
	}
	seen := map[uint]struct{}{}
	for _, id := range cmd.TagIDs {
		if id == 0 {
			return domainCMS.ErrInvalidInput
		}
		if _, ok := seen[id]; ok {
			return domainCMS.ErrInvalidInput
		}
		seen[id] = struct{}{}
		if _, err := s.repo.FindTag(ctx, id); err != nil {
			return mapTag(err)
		}
	}
	return s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.ReplaceArticleTags(ctx, cmd.ArticleID, cmd.TagIDs); err != nil {
			return err
		}
		return s.publishAudit(ctx, cmd.ActorUserID, "article", cmd.ArticleID, domainAudit.ActionCMSArticleTagsChanged, cmd.IP, cmd.UserAgent, map[string]any{"tag_ids": cmd.TagIDs})
	})
}
func (s *Service) CreateLocale(ctx context.Context, cmd CreateLocaleCmd) (*LocaleResult, error) {
	code, name := strings.TrimSpace(cmd.Code), strings.TrimSpace(cmd.Name)
	if !validLocale(code) || name == "" {
		return nil, domainCMS.ErrInvalidInput
	}
	locale := &domainCMS.Locale{Code: code, Name: name, IsEnabled: true, SortOrder: cmd.SortOrder}
	if err := s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.CreateLocale(ctx, locale); err != nil {
			return err
		}
		return s.publishAuditText(ctx, cmd.ActorUserID, "locale", locale.Code, domainAudit.ActionCMSLocaleCreated, cmd.IP, cmd.UserAgent, map[string]any{"name": locale.Name, "sort_order": locale.SortOrder})
	}); err != nil {
		return nil, err
	}
	return localeResult(locale), nil
}
func (s *Service) UpdateLocale(ctx context.Context, cmd UpdateLocaleCmd) (*LocaleResult, error) {
	if !validLocale(strings.TrimSpace(cmd.Code)) || strings.TrimSpace(cmd.Name) == "" {
		return nil, domainCMS.ErrInvalidInput
	}
	locale, err := s.repo.FindLocale(ctx, cmd.Code)
	if err != nil {
		return nil, mapLocale(err)
	}
	if locale.IsDefault && !cmd.IsEnabled {
		return nil, domainCMS.ErrLocaleDefault
	}
	if locale.IsEnabled && !cmd.IsEnabled {
		count, err := s.repo.CountEnabledLocales(ctx)
		if err != nil {
			return nil, err
		}
		if count <= 1 {
			return nil, domainCMS.ErrLastEnabledLocale
		}
	}
	if cmd.IsDefault && !cmd.IsEnabled {
		return nil, domainCMS.ErrInvalidInput
	}
	locale.Name, locale.IsEnabled, locale.SortOrder = strings.TrimSpace(cmd.Name), cmd.IsEnabled, cmd.SortOrder
	oldName, oldEnabled, oldSortOrder, oldDefault := locale.Name, locale.IsEnabled, locale.SortOrder, locale.IsDefault
	if err := s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.UpdateLocale(ctx, locale); err != nil {
			return err
		}
		if cmd.IsDefault && !locale.IsDefault {
			if err := s.repo.SetDefaultLocale(ctx, locale.Code); err != nil {
				return err
			}
			locale.IsDefault = true
		}
		return s.publishAuditText(ctx, cmd.ActorUserID, "locale", locale.Code, domainAudit.ActionCMSLocaleUpdated, cmd.IP, cmd.UserAgent, map[string]any{"old_name": oldName, "new_name": locale.Name, "old_enabled": oldEnabled, "new_enabled": locale.IsEnabled, "old_sort_order": oldSortOrder, "new_sort_order": locale.SortOrder, "old_default": oldDefault, "new_default": locale.IsDefault})
	}); err != nil {
		return nil, err
	}
	return localeResult(locale), nil
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
func (s *Service) UpsertCategoryTranslation(ctx context.Context, cmd UpsertCategoryTranslationCmd) (*CategoryResult, error) {
	if cmd.CategoryID == 0 {
		return nil, domainCMS.ErrInvalidInput
	}
	if err := validNameSlug(cmd.Name, cmd.Slug); err != nil {
		return nil, err
	}
	if err := s.requireExistingLocale(ctx, cmd.Locale); err != nil {
		return nil, err
	}
	category, err := s.repo.FindCategory(ctx, cmd.CategoryID)
	if err != nil {
		return nil, mapCategory(err)
	}
	locale := strings.TrimSpace(cmd.Locale)
	translation := &domainCMS.CategoryTranslation{CategoryID: cmd.CategoryID, Locale: locale, Name: strings.TrimSpace(cmd.Name), Slug: strings.TrimSpace(cmd.Slug), Description: cmd.Description, SEOTitle: cmd.SEOTitle, SEODescription: cmd.SEODescription}
	old, oldErr := s.repo.FindCategoryTranslation(ctx, cmd.CategoryID, locale)
	if oldErr != nil && !errors.Is(oldErr, shared.ErrNotFound) {
		return nil, oldErr
	}
	if err := s.tx.Do(ctx, func(ctx context.Context) error {
		if old != nil && old.Slug != translation.Slug {
			enabled, err := s.repo.LocaleEnabled(ctx, locale)
			if err != nil {
				return err
			}
			if category.Enabled && enabled {
				if err := s.ensureSlugAvailable(ctx, locale, categoryPath(locale, translation.Slug)); err != nil {
					return err
				}
			}
		}
		if err := s.repo.UpsertCategoryTranslation(ctx, translation); err != nil {
			return err
		}
		if old != nil && old.Slug != translation.Slug {
			enabled, err := s.repo.LocaleEnabled(ctx, locale)
			if err != nil {
				return err
			}
			if category.Enabled && enabled {
				redirect := &domainCMS.URLRedirect{Locale: locale, SourcePath: categoryPath(locale, old.Slug), TargetPath: categoryPath(locale, translation.Slug), StatusCode: 301}
				if err := s.repo.SaveURLRedirect(ctx, redirect); err != nil {
					return err
				}
				return s.publishAudit(ctx, cmd.ActorUserID, "category", cmd.CategoryID, domainAudit.ActionCMSSlugChanged, cmd.IP, cmd.UserAgent, map[string]any{"locale": locale, "old_slug": old.Slug, "new_slug": translation.Slug})
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &CategoryResult{ID: category.ID, ParentID: category.ParentID, SortOrder: category.SortOrder, Locale: translation.Locale, Name: translation.Name, Slug: translation.Slug}, nil
}
func (s *Service) UpsertTagTranslation(ctx context.Context, cmd UpsertTagTranslationCmd) (*TagResult, error) {
	if cmd.TagID == 0 {
		return nil, domainCMS.ErrInvalidInput
	}
	if err := validNameSlug(cmd.Name, cmd.Slug); err != nil {
		return nil, err
	}
	if err := s.requireExistingLocale(ctx, cmd.Locale); err != nil {
		return nil, err
	}
	if _, err := s.repo.FindTag(ctx, cmd.TagID); err != nil {
		return nil, mapTag(err)
	}
	tr := &domainCMS.TagTranslation{TagID: cmd.TagID, Locale: strings.TrimSpace(cmd.Locale), Name: strings.TrimSpace(cmd.Name), Slug: strings.TrimSpace(cmd.Slug)}
	old, err := s.repo.FindTagTranslation(ctx, cmd.TagID, tr.Locale)
	if err != nil && !errors.Is(err, shared.ErrNotFound) {
		return nil, err
	}
	if err := s.tx.Do(ctx, func(ctx context.Context) error {
		if old != nil && old.Slug != tr.Slug {
			enabled, e := s.repo.LocaleEnabled(ctx, tr.Locale)
			if e != nil {
				return e
			}
			if enabled {
				if e = s.ensureSlugAvailable(ctx, tr.Locale, tagPath(tr.Locale, tr.Slug)); e != nil {
					return e
				}
			}
		}
		if e := s.repo.UpsertTagTranslation(ctx, tr); e != nil {
			return e
		}
		if old != nil && old.Slug != tr.Slug {
			enabled, e := s.repo.LocaleEnabled(ctx, tr.Locale)
			if e != nil {
				return e
			}
			if enabled {
				return s.repo.SaveURLRedirect(ctx, &domainCMS.URLRedirect{Locale: tr.Locale, SourcePath: tagPath(tr.Locale, old.Slug), TargetPath: tagPath(tr.Locale, tr.Slug), StatusCode: 301})
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return tagResult(cmd.TagID, tr), nil
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
	return s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.MoveCategory(ctx, cmd.CategoryID, cmd.ParentID, cmd.SortOrder); err != nil {
			return err
		}
		return s.publishAudit(ctx, cmd.ActorUserID, "category", cmd.CategoryID, domainAudit.ActionCMSCategoryMoved, cmd.IP, cmd.UserAgent, map[string]any{"parent_id": cmd.ParentID, "sort_order": cmd.SortOrder})
	})
}
func (s *Service) UpdateCategory(ctx context.Context, cmd UpdateCategoryCmd) (*CategoryResult, error) {
	if cmd.CategoryID == 0 {
		return nil, domainCMS.ErrInvalidInput
	}
	category, err := s.repo.FindCategory(ctx, cmd.CategoryID)
	if err != nil {
		return nil, mapCategory(err)
	}
	if err := s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.UpdateCategory(ctx, cmd.CategoryID, cmd.IsEnabled, cmd.SortOrder); err != nil {
			return err
		}
		return s.publishAudit(ctx, cmd.ActorUserID, "category", cmd.CategoryID, domainAudit.ActionCMSCategoryUpdated, cmd.IP, cmd.UserAgent, map[string]any{"old_enabled": category.Enabled, "new_enabled": cmd.IsEnabled, "old_sort_order": category.SortOrder, "new_sort_order": cmd.SortOrder})
	}); err != nil {
		return nil, err
	}
	category.Enabled, category.SortOrder = cmd.IsEnabled, cmd.SortOrder
	return &CategoryResult{ID: category.ID, ParentID: category.ParentID, SortOrder: category.SortOrder}, nil
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
	oldSlug := tr.Slug
	wasPublic := tr.Status == domainCMS.TranslationPublished && tr.PublishedAt != nil && !tr.PublishedAt.After(s.now())
	tr.Title, tr.Slug, tr.Summary, tr.Content, tr.ContentFormat, tr.SEOTitle, tr.SEODescription, tr.CanonicalURL = cmd.Title, cmd.Slug, cmd.Summary, cmd.Content, cmd.ContentFormat, cmd.SEOTitle, cmd.SEODescription, cmd.CanonicalURL
	if err := s.tx.Do(ctx, func(ctx context.Context) error {
		if oldSlug != tr.Slug && wasPublic {
			if err := s.ensureSlugAvailable(ctx, tr.Locale, articlePath(tr.Locale, tr.Slug)); err != nil {
				return err
			}
		}
		if err := s.repo.SaveArticleTranslation(ctx, tr); err != nil {
			return err
		}
		if oldSlug != tr.Slug && wasPublic {
			redirect := &domainCMS.URLRedirect{Locale: tr.Locale, SourcePath: articlePath(tr.Locale, oldSlug), TargetPath: articlePath(tr.Locale, tr.Slug), StatusCode: 301}
			if err := s.repo.SaveURLRedirect(ctx, redirect); err != nil {
				return err
			}
			return s.publishAudit(ctx, cmd.ActorUserID, "article_translation", tr.ID, domainAudit.ActionCMSSlugChanged, cmd.IP, cmd.UserAgent, map[string]any{"article_id": cmd.ArticleID, "locale": tr.Locale, "old_slug": oldSlug, "new_slug": tr.Slug})
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return articleResult(cmd.ArticleID, tr), nil
}
func (s *Service) ResolveRedirect(ctx context.Context, locale, sourcePath string) (*RedirectResult, error) {
	if err := s.requireLocale(ctx, locale); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(sourcePath, "/"+locale+"/") {
		return nil, domainCMS.ErrInvalidInput
	}
	redirect, err := s.repo.FindURLRedirect(ctx, locale, sourcePath)
	if errors.Is(err, shared.ErrNotFound) {
		return nil, domainCMS.ErrRedirectNotFound
	}
	if err != nil {
		return nil, err
	}
	return &RedirectResult{SourcePath: redirect.SourcePath, TargetPath: redirect.TargetPath, StatusCode: redirect.StatusCode}, nil
}
func (s *Service) PublishTranslation(ctx context.Context, cmd PublishTranslationCmd) (*ArticleResult, error) {
	tr, err := s.translation(ctx, cmd.ArticleID, cmd.Locale)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	tr.Status, tr.PublishedAt = domainCMS.TranslationPublished, &now
	if err := s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.SaveArticleTranslation(ctx, tr); err != nil {
			return err
		}
		return s.publishAudit(ctx, cmd.ActorUserID, "article_translation", tr.ID, domainAudit.ActionCMSArticlePublished, cmd.IP, cmd.UserAgent, map[string]any{"article_id": cmd.ArticleID, "locale": cmd.Locale})
	}); err != nil {
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
	if err := s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.SaveArticleTranslation(ctx, tr); err != nil {
			return err
		}
		return s.publishAudit(ctx, cmd.ActorUserID, "article_translation", tr.ID, domainAudit.ActionCMSArticleArchived, cmd.IP, cmd.UserAgent, map[string]any{"article_id": cmd.ArticleID, "locale": cmd.Locale})
	}); err != nil {
		return nil, err
	}
	return articleResult(cmd.ArticleID, tr), nil
}
func (s *Service) DeleteArticle(ctx context.Context, cmd DeleteArticleCmd) error {
	if cmd.ArticleID == 0 {
		return domainCMS.ErrInvalidInput
	}
	article, err := s.repo.FindArticleIncludingDeleted(ctx, cmd.ArticleID)
	if err != nil {
		return mapArticle(err)
	}
	if article.DeletedAt != nil {
		return domainCMS.ErrArticleDeleted
	}
	now := s.now().UTC()
	return s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.SoftDeleteArticle(ctx, cmd.ArticleID, now); err != nil {
			return err
		}
		return s.publishAudit(ctx, cmd.ActorUserID, "article", cmd.ArticleID, domainAudit.ActionCMSArticleDeleted, cmd.IP, cmd.UserAgent, map[string]any{"deleted_at": now})
	})
}
func (s *Service) RestoreArticle(ctx context.Context, cmd RestoreArticleCmd) error {
	if cmd.ArticleID == 0 {
		return domainCMS.ErrInvalidInput
	}
	article, err := s.repo.FindArticleIncludingDeleted(ctx, cmd.ArticleID)
	if err != nil {
		return mapArticle(err)
	}
	if article.DeletedAt == nil {
		return domainCMS.ErrArticleActive
	}
	return s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.RestoreArticle(ctx, cmd.ArticleID); err != nil {
			return err
		}
		return s.publishAudit(ctx, cmd.ActorUserID, "article", cmd.ArticleID, domainAudit.ActionCMSArticleRestored, cmd.IP, cmd.UserAgent, map[string]any{"deleted_at": article.DeletedAt})
	})
}
func (s *Service) ListCategories(ctx context.Context, cmd ListCategoriesCmd) ([]*CategoryTreeResult, error) {
	if err := s.requireLocale(ctx, cmd.Locale); err != nil {
		return nil, err
	}
	items, err := s.repo.ListCategoryTreeItems(ctx, cmd.Locale)
	if err != nil {
		return nil, err
	}
	return categoryTree(items), nil
}
func (s *Service) ListPublishedCategories(ctx context.Context, locale string) ([]*CategoryTreeResult, error) {
	if err := s.requireLocale(ctx, locale); err != nil {
		return nil, err
	}
	items, err := s.repo.ListPublicCategoryTreeItems(ctx, locale)
	if err != nil {
		return nil, err
	}
	return categoryTree(items), nil
}
func categoryTree(items []*domainCMS.CategoryTreeItem) []*CategoryTreeResult {
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
	return roots
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
	items, total, err := s.repo.ListArticleTranslations(ctx, cmd.Locale, cmd.IncludeDeleted, page)
	if err != nil {
		return nil, shared.PageResult{}, err
	}
	results := make([]*ArticleResult, 0, len(items))
	for _, item := range items {
		results = append(results, articleResult(item.Article.ID, &item.ArticleTranslation))
	}
	return results, shared.PageResult{Page: page.Page, PerPage: page.PerPage, Total: total}, nil
}
func (s *Service) GetArticleTranslation(ctx context.Context, cmd GetArticleTranslationCmd) (*ArticleDetailResult, error) {
	if cmd.ArticleID == 0 {
		return nil, domainCMS.ErrInvalidInput
	}
	if err := s.requireExistingLocale(ctx, cmd.Locale); err != nil {
		return nil, err
	}
	article, err := s.repo.FindArticle(ctx, cmd.ArticleID)
	if err != nil {
		return nil, mapArticle(err)
	}
	translation, err := s.translation(ctx, cmd.ArticleID, cmd.Locale)
	if err != nil {
		return nil, err
	}
	categories, err := s.repo.ListArticleCategories(ctx, cmd.ArticleID)
	if err != nil {
		return nil, err
	}
	tags, err := s.repo.ListArticleTags(ctx, cmd.ArticleID, cmd.Locale)
	if err != nil {
		return nil, err
	}
	result := &ArticleDetailResult{ID: article.ID, AuthorUserID: article.AuthorUserID, Locale: translation.Locale, Title: translation.Title, Slug: translation.Slug, Summary: translation.Summary, Content: translation.Content, ContentFormat: translation.ContentFormat, Status: string(translation.Status), PublishedAt: translation.PublishedAt, SEOTitle: translation.SEOTitle, SEODescription: translation.SEODescription, CanonicalURL: translation.CanonicalURL, Categories: make([]ArticleCategoryResult, 0, len(categories)), Tags: make([]TagResult, 0, len(tags))}
	for _, category := range categories {
		result.Categories = append(result.Categories, ArticleCategoryResult{CategoryID: category.CategoryID, IsPrimary: category.IsPrimary})
	}
	for _, tag := range tags {
		result.Tags = append(result.Tags, *tagResult(tag.ID, &tag.TagTranslation))
	}
	return result, nil
}
func (s *Service) GetPublishedArticle(ctx context.Context, locale, slug string) (*PublicArticleResult, error) {
	if err := s.requireLocale(ctx, locale); err != nil {
		return nil, err
	}
	a, err := s.repo.FindPublicArticle(ctx, locale, slug)
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return nil, domainCMS.ErrTranslationAbsent
		}
		return nil, err
	}
	locales, err := s.repo.ListPublishedArticleLocales(ctx, a.Article.ID)
	if err != nil {
		return nil, err
	}
	breadcrumbs, err := s.repo.ListPublicArticleBreadcrumbs(ctx, a.Article.ID, locale)
	if err != nil {
		return nil, err
	}
	result := &PublicArticleResult{ID: a.Article.ID, Locale: a.Locale, Title: a.Title, Slug: a.Slug, Summary: a.Summary, Content: a.Content, ContentFormat: a.ContentFormat, PublishedAt: a.PublishedAt, SEOTitle: a.SEOTitle, SEODescription: a.SEODescription, CanonicalURL: a.CanonicalURL, UpdatedAt: a.ArticleTranslation.UpdatedAt, AvailableLocales: make([]PublicLocaleRef, 0, len(locales)), Breadcrumbs: make([]PublicCategoryRef, 0, len(breadcrumbs))}
	for _, translation := range locales {
		result.AvailableLocales = append(result.AvailableLocales, PublicLocaleRef{Locale: translation.Locale, Slug: translation.Slug})
	}
	for index, category := range breadcrumbs {
		ref := PublicCategoryRef{ID: category.ID, Name: category.Name, Slug: category.Slug}
		result.Breadcrumbs = append(result.Breadcrumbs, ref)
		if index == len(breadcrumbs)-1 {
			result.PrimaryCategory = &ref
		}
	}
	return result, nil
}
func (s *Service) ListPublicSitemapEntries(ctx context.Context, cmd ListPublicSitemapEntriesCmd) ([]*SitemapEntryResult, shared.PageResult, error) {
	if err := s.requireLocale(ctx, cmd.Locale); err != nil {
		return nil, shared.PageResult{}, err
	}
	page := shared.NewPageQuery(cmd.Page.Page, cmd.Page.PerPage)
	entries, total, err := s.repo.ListPublicSitemapEntries(ctx, cmd.Locale, page)
	if err != nil {
		return nil, shared.PageResult{}, err
	}
	results := make([]*SitemapEntryResult, 0, len(entries))
	for _, entry := range entries {
		var path string
		switch entry.Kind {
		case "article":
			path = fmt.Sprintf("/%s/articles/%s", cmd.Locale, entry.Slug)
		case "category":
			path = fmt.Sprintf("/%s/categories/%s", cmd.Locale, entry.Slug)
		default:
			continue
		}
		results = append(results, &SitemapEntryResult{URL: path, LastModified: entry.UpdatedAt})
	}
	return results, shared.PageResult{Page: page.Page, PerPage: page.PerPage, Total: total}, nil
}
func (s *Service) ListPublishedArticles(ctx context.Context, cmd ListPublicArticlesCmd) ([]*PublicArticleListResult, shared.PageResult, error) {
	return s.listPublishedArticles(ctx, cmd.Locale, nil, cmd.Page)
}
func (s *Service) ListPublishedCategoryArticles(ctx context.Context, cmd ListPublicCategoryArticlesCmd) ([]*PublicArticleListResult, shared.PageResult, error) {
	if strings.TrimSpace(cmd.CategorySlug) == "" {
		return nil, shared.PageResult{}, domainCMS.ErrInvalidInput
	}
	if err := s.requireLocale(ctx, cmd.Locale); err != nil {
		return nil, shared.PageResult{}, err
	}
	exists, err := s.repo.PublicCategoryExists(ctx, cmd.Locale, cmd.CategorySlug)
	if err != nil {
		return nil, shared.PageResult{}, err
	}
	if !exists {
		return nil, shared.PageResult{}, domainCMS.ErrCategoryNotFound
	}
	return s.listPublishedArticles(ctx, cmd.Locale, &cmd.CategorySlug, cmd.Page)
}
func (s *Service) ListPublishedTagArticles(ctx context.Context, cmd ListPublicTagArticlesCmd) ([]*PublicArticleListResult, shared.PageResult, error) {
	if strings.TrimSpace(cmd.TagSlug) == "" {
		return nil, shared.PageResult{}, domainCMS.ErrInvalidInput
	}
	if err := s.requireLocale(ctx, cmd.Locale); err != nil {
		return nil, shared.PageResult{}, err
	}
	ok, err := s.repo.PublicTagExists(ctx, cmd.Locale, cmd.TagSlug)
	if err != nil {
		return nil, shared.PageResult{}, err
	}
	if !ok {
		return nil, shared.PageResult{}, domainCMS.ErrTagNotFound
	}
	page := shared.NewPageQuery(cmd.Page.Page, cmd.Page.PerPage)
	items, total, err := s.repo.ListPublicTagArticles(ctx, cmd.Locale, cmd.TagSlug, page)
	if err != nil {
		return nil, shared.PageResult{}, err
	}
	results := make([]*PublicArticleListResult, 0, len(items))
	for _, item := range items {
		results = append(results, &PublicArticleListResult{ID: item.Article.ID, Locale: item.Locale, Title: item.Title, Slug: item.Slug, Summary: item.Summary, ContentFormat: item.ContentFormat, PublishedAt: item.PublishedAt, UpdatedAt: item.ArticleTranslation.UpdatedAt})
	}
	return results, shared.PageResult{Page: page.Page, PerPage: page.PerPage, Total: total}, nil
}
func (s *Service) listPublishedArticles(ctx context.Context, locale string, categorySlug *string, page shared.PageQuery) ([]*PublicArticleListResult, shared.PageResult, error) {
	if err := s.requireLocale(ctx, locale); err != nil {
		return nil, shared.PageResult{}, err
	}
	page = shared.NewPageQuery(page.Page, page.PerPage)
	items, total, err := s.repo.ListPublicArticles(ctx, locale, categorySlug, page)
	if err != nil {
		return nil, shared.PageResult{}, err
	}
	results := make([]*PublicArticleListResult, 0, len(items))
	for _, item := range items {
		result := &PublicArticleListResult{ID: item.Article.ID, Locale: item.Locale, Title: item.Title, Slug: item.Slug, Summary: item.Summary, ContentFormat: item.ContentFormat, PublishedAt: item.PublishedAt, UpdatedAt: item.ArticleTranslation.UpdatedAt}
		if item.PrimaryCategoryID != nil {
			result.PrimaryCategory = &PublicCategoryRef{ID: *item.PrimaryCategoryID, Name: item.PrimaryCategoryName, Slug: item.PrimaryCategorySlug}
		}
		results = append(results, result)
	}
	return results, shared.PageResult{Page: page.Page, PerPage: page.PerPage, Total: total}, nil
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
func (s *Service) requireExistingLocale(ctx context.Context, locale string) error {
	_, err := s.repo.FindLocale(ctx, strings.TrimSpace(locale))
	return mapLocale(err)
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
func mapTag(err error) error {
	if errors.Is(err, shared.ErrNotFound) {
		return domainCMS.ErrTagNotFound
	}
	return err
}
func mapLocale(err error) error {
	if errors.Is(err, shared.ErrNotFound) {
		return domainCMS.ErrLocaleNotFound
	}
	return err
}
func validLocale(code string) bool {
	if len(code) < 2 || len(code) > 35 {
		return false
	}
	for _, r := range code {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}
func (s *Service) publishAudit(ctx context.Context, actorUserID uint, targetType string, targetID uint, action, ip, userAgent string, metadata map[string]any) error {
	return s.publishAuditText(ctx, actorUserID, targetType, strconv.FormatUint(uint64(targetID), 10), action, ip, userAgent, metadata)
}
func (s *Service) publishAuditText(ctx context.Context, actorUserID uint, targetType, targetID, action, ip, userAgent string, metadata map[string]any) error {
	if s.eventBus == nil {
		return nil
	}
	var actor *uint
	if actorUserID != 0 {
		actor = &actorUserID
	}
	return s.eventBus.Publish(ctx, domainAudit.LogRequested{ActorUserID: actor, TargetType: targetType, TargetID: targetID, Action: action, Result: domainAudit.ResultSuccess, IP: ip, UserAgent: userAgent, Metadata: metadata})
}
func (s *Service) ensureSlugAvailable(ctx context.Context, locale, path string) error {
	exists, err := s.repo.RedirectSourceExists(ctx, locale, path)
	if err != nil {
		return err
	}
	if exists {
		return domainCMS.ErrSlugReserved
	}
	return nil
}
func articlePath(locale, slug string) string  { return fmt.Sprintf("/%s/articles/%s", locale, slug) }
func categoryPath(locale, slug string) string { return fmt.Sprintf("/%s/categories/%s", locale, slug) }
func tagPath(locale, slug string) string      { return fmt.Sprintf("/%s/tags/%s", locale, slug) }
func tagResult(id uint, tr *domainCMS.TagTranslation) *TagResult {
	return &TagResult{ID: id, Locale: tr.Locale, Name: tr.Name, Slug: tr.Slug}
}
func localeResult(locale *domainCMS.Locale) *LocaleResult {
	return &LocaleResult{Code: locale.Code, Name: locale.Name, IsDefault: locale.IsDefault, IsEnabled: locale.IsEnabled, SortOrder: locale.SortOrder}
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
