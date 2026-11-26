import { test, expect, Page } from '@playwright/test';

// Test data
const TEST_OTP = '123456';
const ADMIN_USERNAME = 'admin';
const ADMIN_PASSWORD = 'TestAdmin123!';
const ADMIN_EMAIL = 'admin@test.local';

// Helper to get OTP from backend (in real test, would fetch from API)
async function getOTP(page: Page): Promise<string> {
  // In CI, this would be fetched from the backend logs
  // For testing, we'll use a mock endpoint or predefined value
  const response = await page.request.get('/api/setup/otp');
  if (response.ok()) {
    const data = await response.json();
    return data.otp;
  }
  return TEST_OTP;
}

test.describe('First Boot Setup', () => {
  test('should complete first-boot setup flow', async ({ page }) => {
    // Navigate to setup page
    await page.goto('/setup');
    
    // Step 1: OTP Verification
    await expect(page.getByRole('heading', { name: /First-Time Setup/i })).toBeVisible();
    await expect(page.getByText(/Enter the one-time password/i)).toBeVisible();
    
    // Get OTP (in real scenario, from console/logs)
    const otp = await getOTP(page);
    
    // Enter OTP
    const otpInputs = page.locator('input[type="text"][maxlength="1"]');
    const otpChars = otp.split('');
    for (let i = 0; i < otpChars.length; i++) {
      await otpInputs.nth(i).fill(otpChars[i]);
    }
    
    // Verify OTP
    await page.getByRole('button', { name: /Verify/i }).click();
    
    // Wait for next step
    await expect(page.getByRole('heading', { name: /Create Administrator/i })).toBeVisible();
    
    // Step 2: Create Admin Account
    await page.getByLabel(/Username/i).fill(ADMIN_USERNAME);
    await page.getByLabel(/Email/i).fill(ADMIN_EMAIL);
    await page.getByLabel('Password', { exact: true }).fill(ADMIN_PASSWORD);
    await page.getByLabel(/Confirm Password/i).fill(ADMIN_PASSWORD);
    
    // Optional: Enable 2FA
    const enable2FA = page.getByLabel(/Enable two-factor authentication/i);
    if (await enable2FA.isVisible()) {
      await enable2FA.check();
    }
    
    await page.getByRole('button', { name: /Create Admin/i }).click();
    
    // Step 3: System Configuration
    await expect(page.getByRole('heading', { name: /System Configuration/i })).toBeVisible();
    
    // Set hostname
    await page.getByLabel(/Hostname/i).fill('nos-test');
    
    // Set timezone
    await page.getByLabel(/Timezone/i).selectOption('America/New_York');
    
    await page.getByRole('button', { name: /Continue/i }).click();
    
    // Step 4: Network Configuration
    await expect(page.getByRole('heading', { name: /Network Configuration/i })).toBeVisible();
    
    // For testing, we'll keep DHCP
    await page.getByLabel(/DHCP/i).check();
    
    await page.getByRole('button', { name: /Continue/i }).click();
    
    // Step 5: Telemetry (optional)
    if (await page.getByRole('heading', { name: /Telemetry/i }).isVisible()) {
      // Opt out for testing
      await page.getByLabel(/Send anonymous usage data/i).uncheck();
      await page.getByRole('button', { name: /Continue/i }).click();
    }
    
    // Step 6: Setup Complete
    await expect(page.getByRole('heading', { name: /Setup Complete/i })).toBeVisible();
    await expect(page.getByText(/nOS is ready/i)).toBeVisible();
    
    await page.getByRole('button', { name: /Go to Dashboard/i }).click();
    
    // Verify redirect to login
    await expect(page).toHaveURL('/login');
  });
});

test.describe('Login', () => {
  test('should login with admin credentials', async ({ page }) => {
    await page.goto('/login');
    
    await page.getByLabel(/Username/i).fill(ADMIN_USERNAME);
    await page.getByLabel(/Password/i).fill(ADMIN_PASSWORD);
    
    await page.getByRole('button', { name: /Sign In/i }).click();
    
    // Should redirect to dashboard
    await expect(page).toHaveURL('/');
    await expect(page.getByRole('heading', { name: /Dashboard/i })).toBeVisible();
  });
  
  test('should handle invalid credentials', async ({ page }) => {
    await page.goto('/login');
    
    await page.getByLabel(/Username/i).fill('invalid');
    await page.getByLabel(/Password/i).fill('wrong');
    
    await page.getByRole('button', { name: /Sign In/i }).click();
    
    // Should show error
    await expect(page.getByText(/Invalid credentials/i)).toBeVisible();
    
    // Should remain on login page
    await expect(page).toHaveURL('/login');
  });
  
  test('should trigger lockout after too many attempts', async ({ page }) => {
    await page.goto('/login');
    
    // Try to login 6 times with wrong password
    for (let i = 0; i < 6; i++) {
      await page.getByLabel(/Username/i).fill(ADMIN_USERNAME);
      await page.getByLabel(/Password/i).fill('wrong');
      await page.getByRole('button', { name: /Sign In/i }).click();
      
      // Wait a bit between attempts
      await page.waitForTimeout(500);
    }
    
    // Should show lockout message
    await expect(page.getByText(/Too many failed attempts/i)).toBeVisible();
    
    // Login button should be disabled
    await expect(page.getByRole('button', { name: /Sign In/i })).toBeDisabled();
  });
});
