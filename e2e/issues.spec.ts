import { test, expect } from "@playwright/test";
import type { Page } from "@playwright/test";
import { loginAsDefault, createTestApi, e2eIdentity } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Issues", () => {
  let api: TestApiClient;

  async function openNewIssue(page: Page) {
    await page.getByRole("button", { name: "New Issue" }).first().click({ force: true });
    const switchToManual = page.getByRole("button", { name: "Switch to Manual" });
    if (await switchToManual.waitFor({ state: "visible", timeout: 1500 }).then(() => true, () => false)) {
      await switchToManual.click();
    }
  }

  test.beforeEach(async ({ page }, testInfo) => {
    const identity = e2eIdentity(testInfo);
    api = await createTestApi(identity);
    await loginAsDefault(page, identity);
  });

  test.afterEach(async () => {
    if (api) {
      await api.cleanup();
    }
  });

  test("issues page loads with board view", async ({ page }) => {
    await api.createIssue("E2E Board View " + Date.now());
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

  test("can switch from board to list view", async ({ page }) => {
    const title = "E2E List Switch " + Date.now();
    await api.createIssue(title);
    await page.reload();
    await expect(page.getByText("Not started", { exact: true })).toBeVisible();

    // Switch to list view through the view-mode menu. Avoid `text=List`,
    // which can match issue titles.
    await page.getByRole("button", { name: "Board view" }).click();
    await page.getByRole("menuitem", { name: "List" }).click();
    await expect(page.getByText(title)).toBeVisible();
  });

  test("board cards prefer the problem summary over generic workspace boilerplate", async ({ page }) => {
    const title = "E2E Workspace Summary " + Date.now();
    const problem = "Unique problem summary " + Date.now();
    const genericSolution =
      "原始记录没有结构化方案时，先按这条问题补齐负责人、验收标准和下一步；已有方案则按原文推进，并在完成后关闭对应 TODO。";

    await api.createIssue(title, {
      description: [
        "## 问题",
        problem,
        "",
        "## 原因",
        "这条记录来自人工维护的 TODO / backlog 文档，属于「小需求」。它代表 IndustryInsights 当前仍未闭环的业务、产品或交付缺口。",
        "",
        "## 处理方案",
        genericSolution,
      ].join("\n"),
    });

    await page.reload();

    await expect(page.getByText(problem)).toBeVisible();
    await expect(page.getByText(genericSolution)).toBeHidden();
  });

  test("can create a new issue", async ({ page }) => {
    await openNewIssue(page);

    const title = "E2E Created " + Date.now();
    const titleInput = page.getByRole("textbox", { name: "Issue title" });
    await expect(titleInput).toBeVisible();
    await titleInput.fill(title);
    await page.getByRole("button", { name: "Create Issue" }).click();

    await expect(page.getByText("Issue created")).toBeVisible({ timeout: 10000 });
    await expect(
      page.getByRole("region", { name: /Notifications/ }).getByText(title),
    ).toBeVisible();

    await page.getByRole("button", { name: "View issue" }).click();
    await page.waitForURL(/\/issues\/[\w-]+/);
    await expect(page.locator("text=Properties")).toBeVisible();
  });

  test("can navigate to issue detail page", async ({ page }) => {
    // Create a known issue via API so the test controls its own fixture
    const issue = await api.createIssue("E2E Detail Test " + Date.now());

    // Reload to see the new issue
    await page.reload();

    // Navigate to the issue detail. Use a suffix match so the selector works
    // whether the href is legacy `/issues/{id}` or URL-refactored
    // `/{slug}/issues/{id}`.
    const issueLink = page.locator(`a[href$="/issues/${issue.id}"]`);
    await expect(issueLink).toBeVisible({ timeout: 5000 });
    await issueLink.click();

    await page.waitForURL(/\/issues\/[\w-]+/);

    // Should show Properties panel
    await expect(page.locator("text=Properties")).toBeVisible();
    // Should show breadcrumb link back to Issues
    await expect(
      page.locator("a", { hasText: "Issues" }).first(),
    ).toBeVisible();
  });

  test("can dismiss issue creation", async ({ page }) => {
    await openNewIssue(page);

    const titleInput = page.getByRole("textbox", { name: "Issue title" });
    await expect(titleInput).toBeVisible();

    await page.keyboard.press("Escape");

    await expect(titleInput).not.toBeVisible();
    await expect(page.getByRole("button", { name: "New Issue" }).first()).toBeVisible();
  });
});
