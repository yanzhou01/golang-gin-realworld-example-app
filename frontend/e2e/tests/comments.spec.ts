import { test, expect } from '@playwright/test';
import { uniqueUser, registerUser, loginUser, createArticle } from './helpers';

test.describe('Comments', () => {
  const author = uniqueUser('cmtA');
  const commenter = uniqueUser('cmtB');
  let articleSlug = '';

  test.beforeAll(async ({ browser }) => {
    // Author registers and creates the article
    const page = await browser.newPage();
    await registerUser(page, author);
    articleSlug = await createArticle(page, {
      title: `Comment Test Article ${Date.now()}`,
      description: 'For comment testing',
      body: 'Article body for comment tests.',
    });
    await page.close();

    // Register commenter (second user)
    const page2 = await browser.newPage();
    await registerUser(page2, commenter);
    await page2.close();
  });

  test('logged-in user can post a comment', async ({ page }) => {
    await loginUser(page, commenter);
    await page.goto(`/article/${articleSlug}`);

    const commentText = `Test comment ${Date.now()}`;
    await page.locator('[data-test="comment-input"]').fill(commentText);
    await page.locator('[data-test="comment-submit"]').click();

    // Comment should appear in the list
    await expect(
      page.locator('[data-test="comment-item"]', { hasText: commentText }),
    ).toBeVisible({ timeout: 8_000 });
  });

  test('unauthenticated user sees sign-in prompt instead of comment form', async ({ page }) => {
    await page.goto(`/article/${articleSlug}`);
    // No comment textarea for guests
    await expect(page.locator('[data-test="comment-input"]')).not.toBeVisible();
    // Sign in link shown instead
    await expect(page.locator('a[href="/login"]')).toBeVisible();
  });

  test('commenter can delete their own comment', async ({ page }) => {
    await loginUser(page, commenter);
    await page.goto(`/article/${articleSlug}`);

    // Post a comment to delete
    const commentText = `Delete me ${Date.now()}`;
    await page.locator('[data-test="comment-input"]').fill(commentText);
    await page.locator('[data-test="comment-submit"]').click();

    const commentCard = page.locator('[data-test="comment-item"]', { hasText: commentText });
    await expect(commentCard).toBeVisible({ timeout: 8_000 });

    // Delete it
    await commentCard.locator('[data-test="comment-delete-button"]').click();

    // Comment should disappear
    await expect(commentCard).not.toBeVisible({ timeout: 8_000 });
  });

  test('article author sees no delete buttons for comments by others', async ({ page }) => {
    // Author logs in and views the article
    await loginUser(page, author);
    await page.goto(`/article/${articleSlug}`);

    // Wait for comments to load
    await page.waitForTimeout(1000);

    // Commenter's comments should exist but should NOT have delete buttons visible to the author
    const allCommentItems = page.locator('[data-test="comment-item"]');
    const count = await allCommentItems.count();

    if (count > 0) {
      // No delete buttons should be visible since these are not the author's own comments
      const deleteButtons = page.locator('[data-test="comment-delete-button"]');
      await expect(deleteButtons).toHaveCount(0);
    }
  });
});
