import { test, expect } from '@playwright/test';
import { uniqueUser, registerUser, loginUser, createArticle } from './helpers';

test.describe('Profile & Follow', () => {
  const targetUser = uniqueUser('profT');
  const follower = uniqueUser('profF');
  let articleSlug = '';

  test.beforeAll(async ({ browser }) => {
    // Target user registers and creates an article
    const page = await browser.newPage();
    await registerUser(page, targetUser);
    articleSlug = await createArticle(page, {
      title: `Profile Test Article ${Date.now()}`,
      description: 'For profile testing',
      body: 'Profile article body.',
    });
    await page.close();

    // Follower registers
    const page2 = await browser.newPage();
    await registerUser(page2, follower);
    await page2.close();
  });

  test('profile page shows username and their articles', async ({ page }) => {
    await page.goto(`/profile/${targetUser.username}`);
    // Profile info uses data-test="app-header-username"
    await expect(page.locator('[data-test="app-header-username"]')).toContainText(
      targetUser.username,
    );
    // Should show the article they created
    await expect(page.locator('.article-preview').first()).toBeVisible({ timeout: 10_000 });
  });

  test('logged-in user can follow another user from article page', async ({ page }) => {
    await loginUser(page, follower);
    await page.goto(`/article/${articleSlug}`);

    // Follow button has aria-label="Follow author" (appears twice: banner + article-actions)
    const followBtn = page.getByRole('button', { name: 'Follow author' }).first();
    await expect(followBtn).toBeVisible();
    await followBtn.click();

    // Button label changes to "Unfollow author"
    await expect(page.getByRole('button', { name: 'Unfollow author' }).first()).toBeVisible({
      timeout: 8_000,
    });
  });

  test('follower feed shows articles from followed users', async ({ page }) => {
    await loginUser(page, follower);
    await page.goto('/');

    // Click "Your Feed" tab
    await page.locator('.feed-toggle a', { hasText: 'Your Feed' }).click();

    // Should see the article from targetUser
    await expect(
      page.locator('.article-preview').filter({ hasText: targetUser.username }),
    ).toBeVisible({ timeout: 10_000 });
  });

  test('logged-in user can unfollow from article page', async ({ page }) => {
    await loginUser(page, follower);
    await page.goto(`/article/${articleSlug}`);

    // Currently following → button shows "Unfollow author"
    const unfollowBtn = page.getByRole('button', { name: 'Unfollow author' }).first();
    await expect(unfollowBtn).toBeVisible();
    await unfollowBtn.click();

    // Button reverts to "Follow author"
    await expect(page.getByRole('button', { name: 'Follow author' }).first()).toBeVisible({
      timeout: 8_000,
    });
  });
});

test.describe('Favorites', () => {
  const user = uniqueUser('fav');
  const author = uniqueUser('favA');
  let articleSlug = '';

  test.beforeAll(async ({ browser }) => {
    const page = await browser.newPage();
    await registerUser(page, author);
    articleSlug = await createArticle(page, {
      title: `Favorite Test Article ${Date.now()}`,
      description: 'For favorite testing',
      body: 'Favorite article body.',
    });
    await page.close();

    const page2 = await browser.newPage();
    await registerUser(page2, user);
    await page2.close();
  });

  test('user can favorite an article', async ({ page }) => {
    await loginUser(page, user);
    await page.goto(`/article/${articleSlug}`);

    // Article page has two "Favorite article" buttons (banner + article-actions) — use first()
    const favoriteBtn = page.getByRole('button', { name: 'Favorite article' }).first();
    await favoriteBtn.click();

    // Button label changes to "Unfavorite article"
    await expect(page.getByRole('button', { name: 'Unfavorite article' }).first()).toBeVisible({
      timeout: 8_000,
    });
  });

  test('favorited articles appear on user profile favorited tab', async ({ page }) => {
    await loginUser(page, user);
    await page.goto(`/profile/${user.username}`);

    // Click "Favorited Articles" tab
    await page.locator('.articles-toggle a', { hasText: /Favorited/i }).click();

    // Should see the article by the author
    await expect(
      page.locator('.article-preview').filter({ hasText: author.username }),
    ).toBeVisible({ timeout: 10_000 });
  });

  test('user can unfavorite an article', async ({ page }) => {
    await loginUser(page, user);
    await page.goto(`/article/${articleSlug}`);

    // Article is favorited from previous test → shows "Unfavorite article"
    const unfavBtn = page.getByRole('button', { name: 'Unfavorite article' }).first();
    await unfavBtn.click();

    await expect(page.getByRole('button', { name: 'Favorite article' }).first()).toBeVisible({
      timeout: 8_000,
    });
  });
});

test.describe('Settings', () => {
  const user = uniqueUser('set');

  test.beforeAll(async ({ browser }) => {
    const page = await browser.newPage();
    await registerUser(page, user);
    await page.close();
  });

  test('settings page pre-fills current user data', async ({ page }) => {
    await loginUser(page, user);
    // Already on /settings after login
    await expect(page.locator('input[name="username"]')).toHaveValue(user.username);
    await expect(page.locator('input[name="email"]')).toHaveValue(user.email);
  });

  test('update bio reflects on profile page', async ({ page }) => {
    await loginUser(page, user);
    // Already on /settings
    const newBio = `Updated bio at ${Date.now()}`;
    await page.locator('textarea[name="bio"]').fill(newBio);
    await page.getByRole('button', { name: /Update Settings/i }).click();

    // Wait for the settings update action to complete (redirects back to /settings)
    await expect(page).toHaveURL('/settings');
    // Wait a moment for the update to fully propagate
    await page.waitForLoadState('networkidle');

    // Navigate to profile and verify bio is shown
    await page.goto(`/profile/${user.username}`);
    await expect(page.locator('[data-test="app-header-bio"]')).toContainText(newBio, {
      timeout: 10_000,
    });
  });
});
