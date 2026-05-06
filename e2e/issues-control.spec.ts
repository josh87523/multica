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
    await api?.cleanup();
  });

  test("board renders workspace control stage columns", async ({ page }) => {
    await api.createIssue("E2E Control Backlog " + Date.now(), { status: "backlog" });
    await api.createIssue("E2E Control Done " + Date.now(), { status: "done" });
    await page.reload();

    await expect(page.getByText("Design", { exact: true })).toBeVisible();
    await expect(page.getByText("Not started", { exact: true })).toBeVisible();
    await expect(page.getByText("Developing", { exact: true })).toBeVisible();
    await expect(page.getByText("Testing", { exact: true })).toBeVisible();
    await expect(page.getByText("Review", { exact: true })).toBeVisible();
    await expect(page.getByText("Done", { exact: true })).toBeVisible();
    await expect(page.getByText("Pending", { exact: true })).toBeVisible();
    await expect(page.getByText("Hidden columns")).toBeHidden();
  });
});
