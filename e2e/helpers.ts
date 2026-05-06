import { expect, type Page, type TestInfo } from "@playwright/test";
import { TestApiClient } from "./fixtures";

const DEFAULT_E2E_NAME = "E2E User";

export interface E2EIdentity {
  email: string;
  name: string;
  workspaceName: string;
  workspaceSlug: string;
}

function stableHash(input: string): string {
  let hash = 0;
  for (let i = 0; i < input.length; i += 1) {
    hash = (hash * 31 + input.charCodeAt(i)) >>> 0;
  }
  return hash.toString(36);
}

export function e2eIdentity(testInfo: TestInfo): E2EIdentity {
  const titlePath =
    typeof testInfo.titlePath === "function"
      ? testInfo.titlePath()
      : Array.isArray(testInfo.titlePath)
        ? testInfo.titlePath
        : [testInfo.title];
  const raw = [
    ...titlePath,
    `worker-${testInfo.workerIndex}`,
    `parallel-${testInfo.parallelIndex}`,
  ].join("-");
  const suffix = stableHash(raw);
  return {
    email: `e2e+${suffix}@multica.ai`,
    name: DEFAULT_E2E_NAME,
    workspaceName: `E2E Workspace ${suffix}`,
    workspaceSlug: `e2e-workspace-${suffix}`,
  };
}

/**
 * Log in as the default E2E user and ensure the workspace exists first.
 * Authenticates via API (send-code → DB read → verify-code), then injects
 * the token into localStorage so the browser session is authenticated.
 *
 * Returns the E2E workspace slug so callers can build workspace-scoped URLs.
 */
export async function loginAsDefault(page: Page, identity: E2EIdentity): Promise<string> {
  const api = new TestApiClient();
  await api.login(identity.email, identity.name);
  const workspace = await api.ensureWorkspace(
    identity.workspaceName,
    identity.workspaceSlug,
  );

  const token = api.getToken();
  await page.addInitScript((t) => {
    localStorage.setItem("multica_token", t);
  }, token);
  await page.goto(`/${workspace.slug}/issues`);
  await page.waitForURL(/\/issues$/, { timeout: 10000 });
  const startBlank = page.getByRole("button", { name: "Start blank workspace" });
  if (await startBlank.waitFor({ state: "visible", timeout: 1500 }).then(() => true, () => false)) {
    await startBlank.click();
    await expect(startBlank).not.toBeVisible({ timeout: 5000 });
  }
  return workspace.slug;
}

/**
 * Create a TestApiClient logged in as the default E2E user.
 * Call api.cleanup() in afterEach to remove test data created during the test.
 */
export async function createTestApi(identity: E2EIdentity): Promise<TestApiClient> {
  const api = new TestApiClient();
  await api.login(identity.email, identity.name);
  await api.ensureWorkspace(identity.workspaceName, identity.workspaceSlug);
  return api;
}

export async function openWorkspaceMenu(page: Page) {
  // Click the workspace switcher button (has ChevronDown icon)
  await page.locator("aside button").first().click();
  // Wait for dropdown to appear
  await page.locator('[class*="popover"]').waitFor({ state: "visible" });
}
