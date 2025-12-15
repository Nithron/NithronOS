import { test, expect, Page } from '@playwright/test';

// Test fixtures
const ADMIN_USERNAME = 'admin';
const ADMIN_PASSWORD = 'TestAdmin123!';

// Helper to login
async function login(page: Page) {
  await page.goto('/login');
  await page.getByLabel(/Username/i).fill(ADMIN_USERNAME);
  await page.getByLabel(/Password/i).fill(ADMIN_PASSWORD);
  await page.getByRole('button', { name: /Sign In/i }).click();
  await expect(page).toHaveURL('/');
}

test.describe('NithronSync - Device Management', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto('/settings/sync/devices');
  });

  test('should display sync devices page', async ({ page }) => {
    // Check page header
    await expect(page.getByRole('heading', { name: /Sync Devices/i })).toBeVisible();
    
    // Check register button exists
    await expect(page.getByRole('button', { name: /Register Device/i })).toBeVisible();
    
    // Check empty state or devices table
    const devicesTable = page.locator('[data-testid="devices-table"]');
    const emptyState = page.getByText(/No devices registered/i);
    
    // One of these should be visible
    const tableVisible = await devicesTable.isVisible().catch(() => false);
    const emptyVisible = await emptyState.isVisible().catch(() => false);
    expect(tableVisible || emptyVisible).toBeTruthy();
  });

  test('should register a new device', async ({ page }) => {
    // Click register button
    await page.getByRole('button', { name: /Register Device/i }).click();
    
    // Modal should appear
    const modal = page.locator('[role="dialog"]');
    await expect(modal).toBeVisible();
    
    // Fill device details
    await modal.getByLabel(/Device Name/i).fill('E2E Test Device');
    await modal.getByRole('combobox', { name: /Device Type/i }).click();
    await page.getByRole('option', { name: /Windows/i }).click();
    
    // Submit
    await modal.getByRole('button', { name: /Register/i }).click();
    
    // Should show token
    await expect(modal.getByText(/Device Token/i)).toBeVisible();
    await expect(modal.locator('code, input[readonly]')).toBeVisible();
    
    // Copy button should work
    await expect(modal.getByRole('button', { name: /Copy/i })).toBeVisible();
    
    // Close modal
    await modal.getByRole('button', { name: /Done|Close/i }).click();
    
    // Device should appear in list
    await expect(page.getByText('E2E Test Device')).toBeVisible();
  });

  test('should rename a device', async ({ page }) => {
    // Find a device row
    const deviceRow = page.locator('[data-testid="device-row"]').first();
    
    if (await deviceRow.isVisible()) {
      // Click rename/edit button
      await deviceRow.getByRole('button', { name: /Edit|Rename/i }).click();
      
      // Modal should appear
      const modal = page.locator('[role="dialog"]');
      await expect(modal).toBeVisible();
      
      // Change name
      await modal.getByLabel(/Device Name/i).clear();
      await modal.getByLabel(/Device Name/i).fill('Renamed Device');
      
      // Save
      await modal.getByRole('button', { name: /Save/i }).click();
      
      // Success message
      await expect(page.getByText(/Device renamed|updated/i)).toBeVisible();
    }
  });

  test('should revoke a device', async ({ page }) => {
    // Find a device row
    const deviceRow = page.locator('[data-testid="device-row"]').first();
    
    if (await deviceRow.isVisible()) {
      const deviceName = await deviceRow.locator('[data-testid="device-name"]').textContent();
      
      // Click revoke button
      await deviceRow.getByRole('button', { name: /Revoke|Remove/i }).click();
      
      // Confirm dialog should appear
      const confirmDialog = page.locator('[role="alertdialog"]');
      await expect(confirmDialog).toBeVisible();
      await expect(confirmDialog.getByText(/Are you sure/i)).toBeVisible();
      
      // Confirm
      await confirmDialog.getByRole('button', { name: /Confirm|Revoke/i }).click();
      
      // Device should be removed
      await expect(page.getByText(/Device revoked/i)).toBeVisible();
      
      // Device should no longer appear
      if (deviceName) {
        await expect(page.getByText(deviceName)).not.toBeVisible();
      }
    }
  });

  test('should show device statistics', async ({ page }) => {
    // Statistics cards should be visible
    const statsSection = page.locator('[data-testid="sync-stats"]');
    
    if (await statsSection.isVisible()) {
      await expect(statsSection.getByText(/Total Devices/i)).toBeVisible();
      await expect(statsSection.getByText(/Active/i)).toBeVisible();
      await expect(statsSection.getByText(/Last Sync/i)).toBeVisible();
    }
  });

  test('should display QR code for mobile setup', async ({ page }) => {
    // Click register button
    await page.getByRole('button', { name: /Register Device/i }).click();
    
    const modal = page.locator('[role="dialog"]');
    await expect(modal).toBeVisible();
    
    // Fill device details for mobile
    await modal.getByLabel(/Device Name/i).fill('Mobile Test');
    await modal.getByRole('combobox', { name: /Device Type/i }).click();
    await page.getByRole('option', { name: /iOS|Android/i }).first().click();
    
    // Submit
    await modal.getByRole('button', { name: /Register/i }).click();
    
    // Should show QR code option
    await expect(modal.getByRole('button', { name: /Show QR Code/i })).toBeVisible();
    
    // Click to show QR
    await modal.getByRole('button', { name: /Show QR Code/i }).click();
    
    // QR code should be visible
    const qrCode = modal.locator('[data-testid="qr-code"]');
    await expect(qrCode).toBeVisible();
    
    // QR should have an image
    const qrImage = qrCode.locator('canvas, img, svg');
    await expect(qrImage).toBeVisible();
  });
});

test.describe('NithronSync - Settings', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto('/settings/sync');
  });

  test('should display sync settings page', async ({ page }) => {
    // Check page header
    await expect(page.getByRole('heading', { name: /NithronSync|Sync Settings/i })).toBeVisible();
    
    // Check for sync configuration options
    await expect(page.getByText(/Sync Enabled/i)).toBeVisible();
  });

  test('should toggle sync on share', async ({ page }) => {
    // Find a share card
    const shareCard = page.locator('[data-testid="sync-share-card"]').first();
    
    if (await shareCard.isVisible()) {
      // Get current state
      const toggle = shareCard.locator('[role="switch"]');
      const wasEnabled = await toggle.getAttribute('aria-checked') === 'true';
      
      // Toggle it
      await toggle.click();
      
      // State should change
      const isEnabled = await toggle.getAttribute('aria-checked') === 'true';
      expect(isEnabled).not.toBe(wasEnabled);
      
      // Toggle back
      await toggle.click();
    }
  });

  test('should configure bandwidth limit', async ({ page }) => {
    // Find bandwidth settings
    const bandwidthSection = page.locator('[data-testid="bandwidth-settings"]');
    
    if (await bandwidthSection.isVisible()) {
      // Enable bandwidth limiting
      await bandwidthSection.locator('[role="switch"]').click();
      
      // Set limit value
      const input = bandwidthSection.getByRole('spinbutton');
      await input.clear();
      await input.fill('10');
      
      // Save
      await page.getByRole('button', { name: /Save/i }).click();
      
      // Success message
      await expect(page.getByText(/Settings saved/i)).toBeVisible();
    }
  });

  test('should show WebDAV URL', async ({ page }) => {
    // WebDAV access info should be visible
    const webdavSection = page.locator('[data-testid="webdav-info"]');
    
    if (await webdavSection.isVisible()) {
      // Should show URL
      await expect(webdavSection.getByText(/WebDAV URL/i)).toBeVisible();
      await expect(webdavSection.locator('code')).toBeVisible();
      
      // Copy button
      await expect(webdavSection.getByRole('button', { name: /Copy/i })).toBeVisible();
    }
  });

  test('should pause and resume sync', async ({ page }) => {
    const pauseButton = page.getByRole('button', { name: /Pause Sync/i });
    const resumeButton = page.getByRole('button', { name: /Resume Sync/i });
    
    // Check current state
    const canPause = await pauseButton.isVisible();
    const canResume = await resumeButton.isVisible();
    
    if (canPause) {
      await pauseButton.click();
      await expect(page.getByText(/Sync paused/i)).toBeVisible();
      await expect(resumeButton).toBeVisible();
    } else if (canResume) {
      await resumeButton.click();
      await expect(page.getByText(/Sync resumed/i)).toBeVisible();
      await expect(pauseButton).toBeVisible();
    }
  });
});

test.describe('NithronSync - Share Form Integration', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto('/shares');
  });

  test('should show sync option in share form', async ({ page }) => {
    // Click create share or edit existing
    await page.getByRole('button', { name: /Create Share|New Share/i }).click();
    
    const modal = page.locator('[role="dialog"]');
    await expect(modal).toBeVisible();
    
    // Navigate to Sync tab
    await modal.getByRole('tab', { name: /Sync/i }).click();
    
    // Sync options should be visible
    await expect(modal.getByText(/Enable NithronSync/i)).toBeVisible();
    await expect(modal.locator('[data-testid="sync-enabled-toggle"]')).toBeVisible();
  });

  test('should enable sync on new share', async ({ page }) => {
    // Click create share
    await page.getByRole('button', { name: /Create Share|New Share/i }).click();
    
    const modal = page.locator('[role="dialog"]');
    await expect(modal).toBeVisible();
    
    // Fill basic info
    await modal.getByLabel(/Share Name/i).fill('E2E Sync Share');
    await modal.getByLabel(/Path/i).fill('/mnt/pool1/e2e-sync');
    
    // Navigate to Sync tab
    await modal.getByRole('tab', { name: /Sync/i }).click();
    
    // Enable sync
    const syncToggle = modal.locator('[data-testid="sync-enabled-toggle"]');
    await syncToggle.click();
    
    // Configure max size
    const maxSizeInput = modal.getByLabel(/Max Size/i);
    if (await maxSizeInput.isVisible()) {
      await maxSizeInput.fill('10');
    }
    
    // Add exclude pattern
    const excludeInput = modal.getByPlaceholder(/Exclude pattern/i);
    if (await excludeInput.isVisible()) {
      await excludeInput.fill('*.tmp');
      await modal.getByRole('button', { name: /Add/i }).click();
    }
    
    // Don't actually create - just verify the form works
    await modal.getByRole('button', { name: /Cancel/i }).click();
  });
});

test.describe('NithronSync - API Endpoints', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('should fetch devices via API', async ({ page }) => {
    // Intercept API call
    const devicesResponse = await page.request.get('/api/v1/sync/devices', {
      headers: {
        'Cookie': await page.context().cookies().then(c => 
          c.map(cookie => `${cookie.name}=${cookie.value}`).join('; ')
        ),
      },
    });
    
    expect(devicesResponse.ok()).toBeTruthy();
    const data = await devicesResponse.json();
    expect(Array.isArray(data.devices) || data.devices === undefined).toBeTruthy();
  });

  test('should fetch sync shares via API', async ({ page }) => {
    const sharesResponse = await page.request.get('/api/v1/sync/shares', {
      headers: {
        'Cookie': await page.context().cookies().then(c => 
          c.map(cookie => `${cookie.name}=${cookie.value}`).join('; ')
        ),
      },
    });
    
    expect(sharesResponse.ok()).toBeTruthy();
    const data = await sharesResponse.json();
    expect(Array.isArray(data) || data.shares !== undefined).toBeTruthy();
  });

  test('should register device via API', async ({ page }) => {
    const response = await page.request.post('/api/v1/sync/devices/register', {
      headers: {
        'Cookie': await page.context().cookies().then(c => 
          c.map(cookie => `${cookie.name}=${cookie.value}`).join('; ')
        ),
        'Content-Type': 'application/json',
      },
      data: {
        device_name: 'API Test Device',
        device_type: 'linux',
        client_version: '1.0.0',
      },
    });
    
    expect(response.ok()).toBeTruthy();
    const data = await response.json();
    expect(data.device_token).toBeDefined();
    expect(data.device_token.id).toBeDefined();
    expect(data.access_token).toBeDefined();
  });

  test('should authenticate with device token', async ({ page }) => {
    // First register a device
    const registerResponse = await page.request.post('/api/v1/sync/devices/register', {
      headers: {
        'Cookie': await page.context().cookies().then(c => 
          c.map(cookie => `${cookie.name}=${cookie.value}`).join('; ')
        ),
        'Content-Type': 'application/json',
      },
      data: {
        device_name: 'Token Test Device',
        device_type: 'windows',
        client_version: '1.0.0',
      },
    });
    
    const registerData = await registerResponse.json();
    const accessToken = registerData.access_token;
    
    // Use device token to fetch shares
    const sharesResponse = await page.request.get('/api/v1/sync/shares', {
      headers: {
        'Authorization': `Bearer ${accessToken}`,
      },
    });
    
    expect(sharesResponse.ok()).toBeTruthy();
  });
});

test.describe('NithronSync - WebDAV Integration', () => {
  test('should access WebDAV endpoint', async ({ page }) => {
    await login(page);
    
    // First register a device to get token
    const registerResponse = await page.request.post('/api/v1/sync/devices/register', {
      headers: {
        'Cookie': await page.context().cookies().then(c => 
          c.map(cookie => `${cookie.name}=${cookie.value}`).join('; ')
        ),
        'Content-Type': 'application/json',
      },
      data: {
        device_name: 'WebDAV Test Device',
        device_type: 'linux',
        client_version: '1.0.0',
      },
    });
    
    const registerData = await registerResponse.json();
    const accessToken = registerData.access_token;
    
    // Get shares to find a sync-enabled one
    const sharesResponse = await page.request.get('/api/v1/sync/shares', {
      headers: {
        'Authorization': `Bearer ${accessToken}`,
      },
    });
    
    if (sharesResponse.ok()) {
      const shares = await sharesResponse.json();
      
      if (shares.length > 0) {
        const share = shares[0];
        
        // Try PROPFIND on WebDAV
        const webdavResponse = await page.request.fetch(`/dav/${share.id}/`, {
          method: 'PROPFIND',
          headers: {
            'Authorization': `Bearer ${accessToken}`,
            'Depth': '1',
          },
        });
        
        // WebDAV should respond (might be 207 Multi-Status or 404 if empty)
        expect([200, 207, 404]).toContain(webdavResponse.status());
      }
    }
  });
});

test.describe('NithronSync - Error Handling', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('should handle device registration failure', async ({ page }) => {
    await page.goto('/settings/sync/devices');
    
    // Mock API failure
    await page.route('**/api/v1/sync/devices/register', route => {
      route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'Internal server error', code: 'internal_error' }),
      });
    });
    
    // Try to register
    await page.getByRole('button', { name: /Register Device/i }).click();
    
    const modal = page.locator('[role="dialog"]');
    await modal.getByLabel(/Device Name/i).fill('Error Test');
    await modal.getByRole('combobox', { name: /Device Type/i }).click();
    await page.getByRole('option', { name: /Windows/i }).click();
    await modal.getByRole('button', { name: /Register/i }).click();
    
    // Should show error
    await expect(page.getByText(/Failed|Error|Something went wrong/i)).toBeVisible();
  });

  test('should handle rate limiting', async ({ page }) => {
    await page.goto('/settings/sync/devices');
    
    // Mock rate limit response
    await page.route('**/api/v1/sync/devices/register', route => {
      route.fulfill({
        status: 429,
        contentType: 'application/json',
        headers: {
          'Retry-After': '60',
        },
        body: JSON.stringify({ error: 'Too many requests', code: 'rate_limited' }),
      });
    });
    
    // Try to register
    await page.getByRole('button', { name: /Register Device/i }).click();
    
    const modal = page.locator('[role="dialog"]');
    await modal.getByLabel(/Device Name/i).fill('Rate Limit Test');
    await modal.getByRole('combobox', { name: /Device Type/i }).click();
    await page.getByRole('option', { name: /Windows/i }).click();
    await modal.getByRole('button', { name: /Register/i }).click();
    
    // Should show rate limit message
    await expect(page.getByText(/Too many|rate limit|try again/i)).toBeVisible();
  });

  test('should handle unauthorized access', async ({ page }) => {
    // Try to access sync API without proper auth
    const response = await page.request.get('/api/v1/sync/devices', {
      headers: {
        'Authorization': 'Bearer invalid_token',
      },
    });
    
    expect(response.status()).toBe(401);
  });
});

test.describe('NithronSync - Navigation', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('should navigate to sync from settings menu', async ({ page }) => {
    // Open settings menu
    await page.getByRole('button', { name: /Settings/i }).click();
    
    // Click on Sync
    await page.getByRole('menuitem', { name: /Sync|NithronSync/i }).click();
    
    // Should be on sync settings
    await expect(page).toHaveURL(/\/settings\/sync/);
  });

  test('should navigate between sync pages', async ({ page }) => {
    await page.goto('/settings/sync');
    
    // Navigate to devices
    await page.getByRole('link', { name: /Devices|Manage Devices/i }).click();
    await expect(page).toHaveURL(/\/settings\/sync\/devices/);
    
    // Navigate back to settings
    await page.getByRole('link', { name: /Settings|Back/i }).click();
    await expect(page).toHaveURL(/\/settings\/sync/);
  });

  test('should show sync in sidebar navigation', async ({ page }) => {
    // Check sidebar
    const sidebar = page.locator('[data-testid="sidebar"]');
    
    // Expand settings if needed
    const settingsItem = sidebar.getByRole('link', { name: /Settings/i });
    if (await settingsItem.isVisible()) {
      await settingsItem.click();
    }
    
    // Sync should be visible
    await expect(sidebar.getByRole('link', { name: /Sync|NithronSync/i })).toBeVisible();
  });
});

