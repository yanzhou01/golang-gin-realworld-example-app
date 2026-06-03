import { test, expect } from '@playwright/test';
import { uniqueUser, registerUser, loginUser } from './helpers';

test.describe('Authentication', () => {
  const user = uniqueUser('auth');

  test('home page loads and shows Conduit branding', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveTitle(/Conduit/i);
    await expect(page.locator('.navbar-brand')).toContainText('conduit');
  });

  test('register page renders', async ({ page }) => {
    await page.goto('/register');
    await expect(page.getByRole('heading', { name: 'Sign up' })).toBeVisible();
    await expect(page.getByPlaceholder('Username')).toBeVisible();
    await expect(page.getByPlaceholder('Email')).toBeVisible();
    await expect(page.getByPlaceholder('Password')).toBeVisible();
  });

  test('register new user successfully', async ({ page }) => {
    await registerUser(page, user);
    // After redirect to /settings, username appears in nav
    await expect(page.locator('.nav-link', { hasText: user.username })).toBeVisible();
  });

  test('register with duplicate username shows error', async ({ page }) => {
    await page.goto('/register');
    await page.getByPlaceholder('Username').fill(user.username);       // same username
    await page.getByPlaceholder('Email').fill(`dup_${user.email}`);    // different email
    await page.getByPlaceholder('Password').fill(user.password);
    await page.getByRole('button', { name: 'Sign up' }).click();
    // Should show error and stay on /register
    await expect(page.locator('ul.error-messages')).toBeVisible();
    await expect(page).toHaveURL('/register');
  });

  test('register with duplicate email shows error', async ({ page }) => {
    await page.goto('/register');
    await page.getByPlaceholder('Username').fill(`dup_${user.username}`);
    await page.getByPlaceholder('Email').fill(user.email);             // same email
    await page.getByPlaceholder('Password').fill(user.password);
    await page.getByRole('button', { name: 'Sign up' }).click();
    await expect(page.locator('ul.error-messages')).toBeVisible();
    await expect(page).toHaveURL('/register');
  });

  test('login with valid credentials redirects to settings', async ({ page }) => {
    await loginUser(page, user);
    await expect(page.locator('.nav-link', { hasText: user.username })).toBeVisible();
  });

  test('login with wrong password shows error', async ({ page }) => {
    await page.goto('/login');
    await page.getByPlaceholder('Email').fill(user.email);
    await page.getByPlaceholder('Password').fill('wrongpassword');
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(page.locator('ul.error-messages')).toBeVisible();
    await expect(page).toHaveURL('/login');
  });

  test('login with unknown email shows error', async ({ page }) => {
    await page.goto('/login');
    await page.getByPlaceholder('Email').fill('nobody@nowhere.com');
    await page.getByPlaceholder('Password').fill('password123');
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(page.locator('ul.error-messages')).toBeVisible();
    await expect(page).toHaveURL('/login');
  });

  test('logout redirects to login page', async ({ page }) => {
    await loginUser(page, user);
    await page.goto('/settings');
    // Button text is "Or click here to logout."
    await page.getByRole('button', { name: /logout/i }).click();
    // After logout → redirects to /login
    await expect(page).toHaveURL('/login');
  });
});
