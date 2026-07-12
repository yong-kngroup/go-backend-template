package cms

import (
	"context"
	"errors"
	domainCMS "github.com/freeDog-wy/go-backend-template/internal/domain/cms"
	"github.com/freeDog-wy/go-backend-template/internal/domain/shared"
	"testing"
	"time"
)

type testTx struct{}

func (testTx) Do(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type testRepo struct {
	descendant bool
	tr         *domainCMS.ArticleTranslation
	public     *domainCMS.PublicArticle
	tree       []*domainCMS.CategoryTreeItem
	replaced   []uint
}

func (*testRepo) LocaleEnabled(context.Context, string) (bool, error) { return true, nil }
func (*testRepo) CreateCategory(context.Context, *domainCMS.Category, *domainCMS.CategoryTranslation) error {
	return nil
}
func (*testRepo) FindCategory(_ context.Context, id uint) (*domainCMS.Category, error) {
	return &domainCMS.Category{ID: id}, nil
}
func (r *testRepo) IsCategoryDescendant(context.Context, uint, uint) (bool, error) {
	return r.descendant, nil
}
func (*testRepo) MoveCategory(context.Context, uint, *uint, int) error          { return nil }
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
func (*testRepo) CreateArticleTranslation(context.Context, *domainCMS.ArticleTranslation) error {
	return nil
}
func (r *testRepo) FindArticleTranslation(context.Context, uint, string) (*domainCMS.ArticleTranslation, error) {
	if r.tr == nil {
		return nil, shared.ErrNotFound
	}
	return r.tr, nil
}
func (*testRepo) SaveArticleTranslation(context.Context, *domainCMS.ArticleTranslation) error {
	return nil
}
func (r *testRepo) ReplaceArticleCategories(_ context.Context, _ uint, ids []uint, _ *uint) error {
	r.replaced = ids
	return nil
}
func (*testRepo) ListArticleTranslations(context.Context, string, shared.PageQuery) ([]*domainCMS.ArticleListItem, int64, error) {
	return nil, 0, nil
}
func (r *testRepo) FindPublicArticle(context.Context, string, string) (*domainCMS.PublicArticle, error) {
	if r.public == nil {
		return nil, shared.ErrNotFound
	}
	return r.public, nil
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
	zh := &domainCMS.ArticleTranslation{ArticleID: 1, Locale: "zh-CN", Status: domainCMS.TranslationDraft}
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
func TestGetPublishedArticleHidesAbsentTranslation(t *testing.T) {
	svc := New(testTx{}, &testRepo{})
	_, err := svc.GetPublishedArticle(context.Background(), "en-US", "missing")
	if !errors.Is(err, domainCMS.ErrTranslationAbsent) {
		t.Fatalf("error = %v", err)
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
