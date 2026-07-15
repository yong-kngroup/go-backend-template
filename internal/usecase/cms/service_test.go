package cms

import (
	"context"
	"errors"
	domainCMS "github.com/freeDog-wy/go-backend-template/internal/domain/cms"
	domainMedia "github.com/freeDog-wy/go-backend-template/internal/domain/media"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	"testing"
	"time"
)

type testTx struct{}

func (testTx) Do(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type recordingEventBus struct{ events []shared.Event }

func (b *recordingEventBus) Publish(_ context.Context, events ...shared.Event) error {
	b.events = append(b.events, events...)
	return nil
}

type testPublicMediaFinder struct {
	assets []domainMedia.PublicAsset
	ids    []uint
}

func (f *testPublicMediaFinder) ListPublic(_ context.Context, _ string, ids []uint) ([]domainMedia.PublicAsset, error) {
	f.ids = append(f.ids, ids...)
	return f.assets, nil
}

type testRepo struct {
	descendant   bool
	tr           *domainCMS.ArticleTranslation
	public       *domainCMS.PublicArticle
	tree         []*domainCMS.CategoryTreeItem
	replaced     []uint
	publicList   []*domainCMS.PublicArticleListItem
	locale       *domainCMS.Locale
	enabledCount int64
	article      *domainCMS.Article
	locales      []*domainCMS.Locale
	publicTags   []*domainCMS.TagListItem
	redirects    []domainCMS.URLRedirect
}

func (*testRepo) LocaleEnabled(context.Context, string) (bool, error) { return true, nil }
func (r *testRepo) ListLocales(context.Context) ([]*domainCMS.Locale, error) {
	if r.locales != nil {
		return r.locales, nil
	}
	if r.locale == nil {
		return nil, nil
	}
	return []*domainCMS.Locale{r.locale}, nil
}
func (r *testRepo) FindLocale(_ context.Context, _ string) (*domainCMS.Locale, error) {
	if r.locale == nil {
		return &domainCMS.Locale{Code: "zh-CN", Name: "Chinese", IsEnabled: true}, nil
	}
	return r.locale, nil
}
func (*testRepo) CreateLocale(context.Context, *domainCMS.Locale) error { return nil }
func (*testRepo) UpdateLocale(context.Context, *domainCMS.Locale) error { return nil }
func (*testRepo) SetDefaultLocale(context.Context, string) error        { return nil }
func (r *testRepo) CountEnabledLocales(context.Context) (int64, error) {
	if r.enabledCount == 0 {
		return 2, nil
	}
	return r.enabledCount, nil
}
func (*testRepo) CreateTag(context.Context, *domainCMS.Tag, *domainCMS.TagTranslation) error {
	return nil
}
func (*testRepo) FindTag(_ context.Context, id uint) (*domainCMS.Tag, error) {
	return &domainCMS.Tag{ID: id}, nil
}
func (*testRepo) FindTagTranslation(context.Context, uint, string) (*domainCMS.TagTranslation, error) {
	return nil, shared.ErrNotFound
}
func (*testRepo) UpsertTagTranslation(context.Context, *domainCMS.TagTranslation) error { return nil }
func (*testRepo) ListTags(context.Context, string, shared.PageQuery) ([]*domainCMS.TagListItem, int64, error) {
	return nil, 0, nil
}
func (*testRepo) CreateCategory(context.Context, *domainCMS.Category, *domainCMS.CategoryTranslation) error {
	return nil
}
func (*testRepo) UpsertCategoryTranslation(context.Context, *domainCMS.CategoryTranslation) error {
	return nil
}
func (*testRepo) FindCategoryTranslation(context.Context, uint, string) (*domainCMS.CategoryTranslation, error) {
	return nil, shared.ErrNotFound
}
func (*testRepo) FindCategory(_ context.Context, id uint) (*domainCMS.Category, error) {
	return &domainCMS.Category{ID: id}, nil
}
func (r *testRepo) IsCategoryDescendant(context.Context, uint, uint) (bool, error) {
	return r.descendant, nil
}
func (*testRepo) MoveCategory(context.Context, uint, *uint, int) error          { return nil }
func (*testRepo) UpdateCategory(context.Context, uint, bool, int) error         { return nil }
func (*testRepo) ListCategories(context.Context) ([]*domainCMS.Category, error) { return nil, nil }
func (r *testRepo) ListCategoryTreeItems(context.Context, string) ([]*domainCMS.CategoryTreeItem, error) {
	return r.tree, nil
}
func (*testRepo) CreateArticle(context.Context, *domainCMS.Article, *domainCMS.ArticleTranslation) error {
	return nil
}
func (*testRepo) FindArticle(_ context.Context, id uint) (*domainCMS.Article, error) {
	return &domainCMS.Article{ID: id}, nil
}
func (*testRepo) SetArticleCover(context.Context, uint, *uint) error { return nil }
func (r *testRepo) FindArticleIncludingDeleted(_ context.Context, id uint) (*domainCMS.Article, error) {
	if r.article != nil {
		return r.article, nil
	}
	return &domainCMS.Article{ID: id}, nil
}
func (*testRepo) SoftDeleteArticle(context.Context, uint, time.Time) error { return nil }
func (*testRepo) RestoreArticle(context.Context, uint) error               { return nil }
func (*testRepo) CreateArticleTranslation(context.Context, *domainCMS.ArticleTranslation) error {
	return nil
}
func (r *testRepo) FindArticleTranslation(context.Context, uint, string) (*domainCMS.ArticleTranslation, error) {
	if r.tr == nil {
		return nil, shared.ErrNotFound
	}
	return r.tr, nil
}
func (*testRepo) RedirectSourceExists(context.Context, string, string) (bool, error) {
	return false, nil
}
func (*testRepo) SaveURLRedirect(context.Context, *domainCMS.URLRedirect) error { return nil }
func (*testRepo) FindURLRedirect(context.Context, string, string) (*domainCMS.URLRedirect, error) {
	return nil, shared.ErrNotFound
}
func (r *testRepo) ListURLRedirects(context.Context, string, shared.PageQuery) ([]domainCMS.URLRedirect, int64, error) {
	return r.redirects, int64(len(r.redirects)), nil
}
func (*testRepo) ListArticleCategories(context.Context, uint) ([]domainCMS.ArticleCategory, error) {
	return nil, nil
}
func (*testRepo) ListArticleTags(context.Context, uint, string) ([]*domainCMS.TagListItem, error) {
	return nil, nil
}
func (*testRepo) ReplaceArticleTags(context.Context, uint, []uint) error { return nil }
func (*testRepo) SaveArticleTranslation(context.Context, *domainCMS.ArticleTranslation) error {
	return nil
}
func (r *testRepo) ReplaceArticleCategories(_ context.Context, _ uint, ids []uint, _ *uint) error {
	r.replaced = ids
	return nil
}
func (*testRepo) ListArticleTranslations(context.Context, string, bool, shared.PageQuery) ([]*domainCMS.ArticleListItem, int64, error) {
	return nil, 0, nil
}
func (r *testRepo) FindPublicArticle(context.Context, string, string) (*domainCMS.PublicArticle, error) {
	if r.public == nil {
		return nil, shared.ErrNotFound
	}
	return r.public, nil
}
func (*testRepo) ListPublishedArticleLocales(context.Context, uint) ([]domainCMS.PublishedLocale, error) {
	return nil, nil
}
func (*testRepo) ListPublicArticleBreadcrumbs(context.Context, uint, string) ([]domainCMS.CategoryTreeItem, error) {
	return nil, nil
}
func (*testRepo) ListPublicSitemapEntries(context.Context, string, shared.PageQuery) ([]domainCMS.SitemapEntry, int64, error) {
	return nil, 0, nil
}
func (r *testRepo) ListPublicCategoryTreeItems(context.Context, string) ([]*domainCMS.CategoryTreeItem, error) {
	return r.tree, nil
}
func (*testRepo) PublicCategoryExists(context.Context, string, string) (bool, error) {
	return true, nil
}
func (r *testRepo) ListPublicArticles(context.Context, string, *string, shared.PageQuery) ([]*domainCMS.PublicArticleListItem, int64, error) {
	return r.publicList, int64(len(r.publicList)), nil
}
func (*testRepo) PublicTagExists(context.Context, string, string) (bool, error) { return true, nil }
func (r *testRepo) ListPublicTags(context.Context, string, shared.PageQuery) ([]*domainCMS.TagListItem, int64, error) {
	return r.publicTags, int64(len(r.publicTags)), nil
}
func (*testRepo) ListPublicTagArticles(context.Context, string, string, shared.PageQuery) ([]*domainCMS.PublicArticleListItem, int64, error) {
	return nil, 0, nil
}
func TestListPublishedLocalesExcludesDisabledLocales(t *testing.T) {
	svc := New(testTx{}, &testRepo{locales: []*domainCMS.Locale{
		{Code: "zh-CN", Name: "Chinese", IsEnabled: true, IsDefault: true, SortOrder: 1},
		{Code: "en-US", Name: "English", IsEnabled: false, SortOrder: 2},
	}})
	locales, err := svc.ListPublishedLocales(context.Background())
	if err != nil || len(locales) != 1 || locales[0].Code != "zh-CN" || !locales[0].IsDefault {
		t.Fatalf("locales = %#v, err = %v", locales, err)
	}
}

func TestListPublishedTagsAndRedirects(t *testing.T) {
	repo := &testRepo{
		publicTags: []*domainCMS.TagListItem{{Tag: domainCMS.Tag{ID: 3}, TagTranslation: domainCMS.TagTranslation{TagID: 3, Locale: "zh-CN", Name: "Go", Slug: "go"}}},
		redirects:  []domainCMS.URLRedirect{{Locale: "zh-CN", SourcePath: "/zh-CN/articles/old", TargetPath: "/zh-CN/articles/new", StatusCode: 301}},
	}
	svc := New(testTx{}, repo)
	tags, tagPage, err := svc.ListPublishedTags(context.Background(), ListPublicTagsCmd{Locale: "zh-CN", Page: shared.NewPageQuery(1, 10)})
	if err != nil || len(tags) != 1 || tags[0].Slug != "go" || tagPage.Total != 1 {
		t.Fatalf("tags = %#v, page = %#v, err = %v", tags, tagPage, err)
	}
	redirects, redirectPage, err := svc.ListPublicRedirects(context.Background(), ListPublicRedirectsCmd{Locale: "zh-CN", Page: shared.NewPageQuery(1, 10)})
	if err != nil || len(redirects) != 1 || redirects[0].SourcePath != "/zh-CN/articles/old" || redirectPage.Total != 1 {
		t.Fatalf("redirects = %#v, page = %#v, err = %v", redirects, redirectPage, err)
	}
}
func TestMoveCategoryRejectsDescendantAsParent(t *testing.T) {
	repo := &testRepo{descendant: true}
	svc := New(testTx{}, repo)
	parent := uint(2)
	err := svc.MoveCategory(context.Background(), MoveCategoryCmd{CategoryID: 1, ParentID: &parent})
	if !errors.Is(err, domainCMS.ErrCategoryCycle) {
		t.Fatalf("error = %v, want category cycle", err)
	}
}
func TestPublishTranslationOnlyChangesRequestedLanguage(t *testing.T) {
	zh := &domainCMS.ArticleTranslation{ArticleID: 1, Locale: "zh-CN", Title: "Published article", Slug: "published-article", Content: "Published content", ContentFormat: "markdown", Status: domainCMS.TranslationDraft}
	repo := &testRepo{tr: zh}
	svc := New(testTx{}, repo)
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	svc.now = func() time.Time { return now }
	got, err := svc.PublishTranslation(context.Background(), PublishTranslationCmd{ArticleID: 1, Locale: "zh-CN"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "published" || zh.PublishedAt == nil || !zh.PublishedAt.Equal(now) {
		t.Fatalf("translation = %#v", zh)
	}
}

func TestPreviewPublishSeparatesBlockingChecksFromWarnings(t *testing.T) {
	repo := &testRepo{tr: &domainCMS.ArticleTranslation{
		ArticleID: 1, Locale: "zh-CN", Title: "Article", Slug: "article", ContentFormat: "markdown",
	}}
	preview, err := New(testTx{}, repo).PreviewPublish(context.Background(), PreviewPublishCmd{ArticleID: 1, Locale: "zh-CN"})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Publishable {
		t.Fatalf("preview = %#v, want blocked publication", preview)
	}
	checks := make(map[string]PublishCheck, len(preview.Checks))
	for _, check := range preview.Checks {
		checks[check.Name] = check
	}
	if content := checks["content"]; content.Passed || !content.Blocking {
		t.Fatalf("content check = %#v, want failed blocking check", content)
	}
	if seoTitle := checks["seo_title"]; seoTitle.Passed || seoTitle.Blocking {
		t.Fatalf("seo title check = %#v, want failed warning", seoTitle)
	}
}

func TestPublishTranslationRejectsFailedPublicationChecks(t *testing.T) {
	translation := &domainCMS.ArticleTranslation{ArticleID: 1, Locale: "zh-CN", Title: "Article", Slug: "article", ContentFormat: "markdown", Status: domainCMS.TranslationDraft}
	repo := &testRepo{tr: translation}
	bus := &recordingEventBus{}
	_, err := New(testTx{}, repo, bus).PublishTranslation(context.Background(), PublishTranslationCmd{ArticleID: 1, Locale: "zh-CN"})
	if !errors.Is(err, domainCMS.ErrPublicationNotReady) {
		t.Fatalf("error = %v, want publication not ready", err)
	}
	if translation.Status != domainCMS.TranslationDraft || translation.PublishedAt != nil {
		t.Fatalf("translation mutated after rejected publication: %#v", translation)
	}
	if len(bus.events) != 0 {
		t.Fatalf("events = %#v, want no audit event", bus.events)
	}
}
func TestGetPublishedArticleHidesAbsentTranslation(t *testing.T) {
	svc := New(testTx{}, &testRepo{})
	_, err := svc.GetPublishedArticle(context.Background(), "en-US", "missing")
	if !errors.Is(err, domainCMS.ErrTranslationAbsent) {
		t.Fatalf("error = %v", err)
	}
}
func TestGetPublishedArticleIncludesPublicCover(t *testing.T) {
	coverID := uint(12)
	repo := &testRepo{public: &domainCMS.PublicArticle{
		Article:            domainCMS.Article{ID: 7, CoverMediaID: &coverID},
		ArticleTranslation: domainCMS.ArticleTranslation{ArticleID: 7, Locale: "zh-CN", Title: "Article", Slug: "article"},
	}}
	media := &testPublicMediaFinder{assets: []domainMedia.PublicAsset{{ID: coverID, URL: "https://cdn.example.com/covers/article.png", AltText: "Article cover", Title: "Cover"}}}
	svc := New(testTx{}, repo)
	svc.SetPublicMediaFinder(media)
	result, err := svc.GetPublishedArticle(context.Background(), "zh-CN", "article")
	if err != nil {
		t.Fatal(err)
	}
	if result.Cover == nil || result.Cover.ID != coverID || result.Cover.AltText != "Article cover" {
		t.Fatalf("cover = %#v", result.Cover)
	}
	if len(media.ids) != 1 || media.ids[0] != coverID {
		t.Fatalf("requested media IDs = %#v", media.ids)
	}
}
func TestListCategoriesBuildsTree(t *testing.T) {
	rootID := uint(1)
	repo := &testRepo{tree: []*domainCMS.CategoryTreeItem{
		{Category: domainCMS.Category{ID: rootID}, CategoryTranslation: domainCMS.CategoryTranslation{Name: "Root", Slug: "root"}},
		{Category: domainCMS.Category{ID: 2, ParentID: &rootID}, CategoryTranslation: domainCMS.CategoryTranslation{Name: "Child", Slug: "child"}},
	}}
	result, err := New(testTx{}, repo).ListCategories(context.Background(), ListCategoriesCmd{Locale: "zh-CN"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || len(result[0].Children) != 1 || result[0].Children[0].ID != 2 {
		t.Fatalf("tree = %#v", result)
	}
}
func TestReplaceArticleCategoriesRequiresPrimaryWithinCategories(t *testing.T) {
	primary := uint(3)
	repo := &testRepo{}
	err := New(testTx{}, repo).ReplaceArticleCategories(context.Background(), ReplaceArticleCategoriesCmd{ArticleID: 1, CategoryIDs: []uint{1, 2}, PrimaryCategoryID: &primary})
	if !errors.Is(err, domainCMS.ErrInvalidInput) {
		t.Fatalf("error = %v", err)
	}
	if len(repo.replaced) != 0 {
		t.Fatalf("repository called with %#v", repo.replaced)
	}
}
func TestListPublishedArticlesReturnsOnlySummaryFields(t *testing.T) {
	categoryID := uint(4)
	coverID := uint(8)
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	repo := &testRepo{publicList: []*domainCMS.PublicArticleListItem{{
		Article:            domainCMS.Article{ID: 9, CoverMediaID: &coverID},
		ArticleTranslation: domainCMS.ArticleTranslation{Locale: "zh-CN", Title: "Published", Slug: "published", Summary: "Summary", Content: "must not be returned", ContentFormat: "markdown", PublishedAt: &now, UpdatedAt: now},
		PrimaryCategoryID:  &categoryID, PrimaryCategoryName: "News", PrimaryCategorySlug: "news",
	}}}
	svc := New(testTx{}, repo)
	svc.SetPublicMediaFinder(&testPublicMediaFinder{})
	results, page, err := svc.ListPublishedArticles(context.Background(), ListPublicArticlesCmd{Locale: "zh-CN", Page: shared.NewPageQuery(1, 20)})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Cover != nil || results[0].PrimaryCategory == nil || results[0].PrimaryCategory.Slug != "news" || page.Total != 1 {
		t.Fatalf("result = %#v, page = %#v", results, page)
	}
}
func TestUpdateLocaleRejectsDisablingDefault(t *testing.T) {
	repo := &testRepo{locale: &domainCMS.Locale{Code: "zh-CN", Name: "Chinese", IsDefault: true, IsEnabled: true}}
	_, err := New(testTx{}, repo).UpdateLocale(context.Background(), UpdateLocaleCmd{Code: "zh-CN", Name: "Chinese", IsEnabled: false})
	if !errors.Is(err, domainCMS.ErrLocaleDefault) {
		t.Fatalf("error = %v", err)
	}
}
func TestUpdateLocaleRejectsDisablingLastEnabledLocale(t *testing.T) {
	repo := &testRepo{locale: &domainCMS.Locale{Code: "en-US", Name: "English", IsEnabled: true}, enabledCount: 1}
	_, err := New(testTx{}, repo).UpdateLocale(context.Background(), UpdateLocaleCmd{Code: "en-US", Name: "English", IsEnabled: false})
	if !errors.Is(err, domainCMS.ErrLastEnabledLocale) {
		t.Fatalf("error = %v", err)
	}
}
func TestDeleteArticleWritesAuditEvent(t *testing.T) {
	bus := &recordingEventBus{}
	repo := &testRepo{article: &domainCMS.Article{ID: 7}}
	svc := New(testTx{}, repo, bus)
	svc.now = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	if err := svc.DeleteArticle(context.Background(), DeleteArticleCmd{ArticleID: 7, ActorUserID: 3, IP: "127.0.0.1"}); err != nil {
		t.Fatal(err)
	}
	if len(bus.events) != 1 || bus.events[0].EventName() != "audit.log.requested" {
		t.Fatalf("events = %#v", bus.events)
	}
}
func TestRestoreArticleRequiresDeletedArticle(t *testing.T) {
	err := New(testTx{}, &testRepo{article: &domainCMS.Article{ID: 7}}).RestoreArticle(context.Background(), RestoreArticleCmd{ArticleID: 7})
	if !errors.Is(err, domainCMS.ErrArticleActive) {
		t.Fatalf("error = %v", err)
	}
}
