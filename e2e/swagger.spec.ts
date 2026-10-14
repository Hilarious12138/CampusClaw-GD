import { test, expect } from "@playwright/test";

test("Swagger 浏览器可用 Bearer 试调本项目接口", async ({ page, request }) => {
  test.setTimeout(60000);
  const origin = process.env.APP_ORIGIN!;
  const response = await request.post("/api/auth/login", {
    headers: { Origin: origin },
    data: { username: "student_a", password: process.env.SEED_STUDENT_A_PASSWORD! },
  });
  expect(response.status()).toBe(200);
  const token = (await response.json()).access_token as string;
  await page.goto("/swagger/");
  await expect(page.getByText("CampusClaw-GD API")).toBeVisible();
  await page.getByRole("button", { name: "Authorize" }).first().click();
  await page.locator(".auth-container input").fill(token);
  await page.locator(".dialog-ux button[type=submit]").click();
  await page.locator("button.close-modal").click();
  const endpoint = page.locator(".opblock").filter({ hasText: "/api/me" }).first();
  await endpoint.locator(".opblock-summary").click();
  await endpoint.getByRole("button", { name: "Try it out" }).click();
  const delivered = page.waitForResponse(r => r.url().endsWith("/api/me") && r.request().method() === "GET");
  await endpoint.getByRole("button", { name: "Execute" }).click();
  const actual = await delivered;
  expect(actual.status()).toBe(200);
  expect(actual.request().headers()["authorization"]?.startsWith("Bearer ")).toBeTruthy();
  await expect(endpoint.getByText("Response body")).toBeVisible();
});
