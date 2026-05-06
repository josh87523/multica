import { expect, test } from "@playwright/test";
import { createTestApi, e2eIdentity, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Workspace control issues", () => {
  let api: TestApiClient;

  test.beforeEach(async ({ page }, testInfo) => {
    const identity = e2eIdentity(testInfo);
    api = await createTestApi(identity);
    await loginAsDefault(page, identity);
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("board renders active columns and reports hidden column count", async ({ page }) => {
    await api.createIssue("E2E Control Backlog " + Date.now(), { status: "backlog" });
    await api.createIssue("E2E Control Done " + Date.now(), { status: "done" });
    await page.reload();

    await expect(page.getByText("Todo")).toBeVisible();
    await expect(page.getByText("In Progress")).toBeVisible();
    await expect(page.getByText("Hidden columns (2)")).toBeVisible();
  });
});
