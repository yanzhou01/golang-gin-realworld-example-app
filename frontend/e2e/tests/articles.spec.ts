import { test, expect } from '@playwright/test';
import { uniqueUser, registerUser, loginUser, createArticle } from './helpers';

test.describe('Articles', () => {
  const user = uniqueUser('art');
  const articleTitle = `Test Article ${Date.now()}`;
  let articleSlug = '';

  test.beforeAll(async ({ browser }) => {
    const page = await browser.newPage();
    await registerUser(page, user);
    await page.close();
  });

  test('create a new article', async ({ page }) => {
    await loginUser(page, user);
    articleSlug = await createArticle(page, {
      title: articleTitle,
      description: 'A test article description',
      body: 'This is the article body content.',
      tags: 'playwright testing',
    });
    expect(articleSlug).toBeTruthy();

    // Article page should show title and body
    await expect(page.locator('.banner h1')).toContainText(articleTitle);
    await expect(page.locator('.article-content')).toContainText('This is the article body content.');
  });

  test('view article page shows tags', async ({ page }) => {
    await page.goto(`/article/${articleSlug}`);
    // Tags appear in the tag-list
    await expect(page.locator('.tag-list .tag-pill').first()).toBeVisible({ timeout: 8_000 });
  });

  test('author sees Edit and Delete buttons on their own article', async ({ page }) => {
    await loginUser(page, user);
    await page.goto(`/article/${articleSlug}`);
    // Both links/buttons appear twice (banner + article-actions), use first()
    await expect(page.locator('a.btn', { hasText: 'Edit Article' }).first()).toBeVisible();
    await expect(page.getByRole('button', { name: /Delete Article/i }).first()).toBeVisible();
  });

  test('home page global feed shows articles', async ({ page }) => {
    await page.goto('/');
    // Wait for at least one article-preview card to appear
    await expect(page.locator('.article-preview').first()).toBeVisible({ timeout: 10_000 });
  });

  test('edit an existing article updates the title', async ({ page }) => {
    await loginUser(page, user);
    await page.goto(`/article/${articleSlug}`);
    await page.locator('a.btn', { hasText: 'Edit Article' }).first().click();
    await expect(page).toHaveURL(`/editor/${articleSlug}`);

    const updatedTitle = `${articleTitle} (edited)`;
    await page.getByPlaceholder('Article Title').fill(updatedTitle);
    await page.getByRole('button', { name: 'Publish Article' }).click();

    // Redirects to article page (slug may change after title edit)
    await expect(page).toHaveURL(/\/article\/.+/);
    await expect(page.locator('.banner h1')).toContainText(updatedTitle);

    // Update slug for subsequent tests
    articleSlug = page.url().split('/article/')[1];
  });

  test('delete an article redirects to home', async ({ page }) => {
    await loginUser(page, user);
    await page.goto(`/article/${articleSlug}`);

    // article-delete action redirects to "/" (may include query params like ?limit=10&offset=0)
    await page.getByRole('button', { name: /Delete Article/i }).first().click();
    await expect(page).toHaveURL(/^http:\/\/localhost:3001\/(\?.*)?$/, { timeout: 8_000 });
  });

  test('deleted article is no longer accessible', async ({ page }) => {
    await page.goto(`/article/${articleSlug}`);
    // The article loader throws a 404-like error; React Suspense+Await renders the error card
    // AsyncErrorCard shows "Could not load article" as its title
    await expect(page.locator('text=Could not load article')).toBeVisible({ timeout: 8_000 });
  });
});
