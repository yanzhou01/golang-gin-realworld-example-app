import { type Page, expect } from '@playwright/test';

/** Unique suffix for each test run to avoid username/email collisions in the shared DB */
const RUN_ID = Date.now();

export function uniqueUser(prefix = 'e2e') {
  return {
    username: `${prefix}${RUN_ID}`,
    email: `${prefix}${RUN_ID}@example.com`,
    password: 'password123',
  };
}

/**
 * Register via the UI. Redirects to /settings on success.
 */
export async function registerUser(
  page: Page,
  user: { username: string; email: string; password: string },
) {
  await page.goto('/register');
  await page.getByPlaceholder('Username').fill(user.username);
  await page.getByPlaceholder('Email').fill(user.email);
  await page.getByPlaceholder('Password').fill(user.password);
  await page.getByRole('button', { name: 'Sign up' }).click();
  // After successful registration, the app redirects to /settings
  await expect(page).toHaveURL('/settings');
}

/**
 * Login via the UI. Redirects to /settings on success.
 */
export async function loginUser(
  page: Page,
  user: { email: string; password: string },
) {
  await page.goto('/login');
  await page.getByPlaceholder('Email').fill(user.email);
  await page.getByPlaceholder('Password').fill(user.password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  // After successful login, the app redirects to /settings
  await expect(page).toHaveURL('/settings');
}

/**
 * Create an article via the UI and return its slug (from the URL).
 */
export async function createArticle(
  page: Page,
  opts: { title: string; description: string; body: string; tags?: string },
) {
  await page.goto('/editor');
  await page.getByPlaceholder('Article Title').fill(opts.title);
  await page.getByPlaceholder("What's this article about?").fill(opts.description);
  await page.getByPlaceholder('Write your article (in markdown)').fill(opts.body);
  if (opts.tags) {
    await page.getByPlaceholder('Enter tags').fill(opts.tags);
  }
  await page.getByRole('button', { name: 'Publish Article' }).click();

  // After publish, redirected to /article/<slug>
  await expect(page).toHaveURL(/\/article\/.+/);
  const url = page.url();
  const slug = url.split('/article/')[1];
  return slug;
}
