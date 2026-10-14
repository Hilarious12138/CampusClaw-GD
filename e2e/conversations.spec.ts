import { test, expect } from "@playwright/test";
import { mkdirSync } from "node:fs";

async function signIn(page: import("@playwright/test").Page, username: string, password: string) {
  await page.goto("/");
  await page.getByPlaceholder("输入你的账号").fill(username);
  await page.getByPlaceholder("输入密码").fill(password);
  await page.getByRole("button", { name: "登录 →" }).click();
  await expect(page.getByRole("button", { name: /解题助手/ })).toBeVisible();
}

test("学生选择智能体、流式回答并重新登录读取历史", async ({ page }) => {
  test.setTimeout(180000);
  const sent: string[] = [];
  page.on("request", request => {
    if (request.url().includes("/api/conversations") && request.method() === "POST")
      sent.push(request.headers()["authorization"]?.startsWith("Bearer ") ? "bearer" : "missing");
  });
  await signIn(page, "student_a", process.env.SEED_STUDENT_A_PASSWORD!);
  await page.getByRole("button", { name: /解题助手/ }).click();
  await expect(page.getByLabel("会话智能体")).toContainText("数学助手");
  await page.getByLabel("会话智能体").selectOption({ label: "数学助手" });
  await page.getByRole("button", { name: "＋ 新建会话" }).click();
  await expect(page.getByLabel("会话问题")).toBeVisible();
  await page.getByLabel("会话问题").fill("方程中为什么要保持等式两边相等？只给我下一步提示。");
  await page.getByRole("button", { name: "发送问题" }).click();
  const reply = page.locator(".chat-message.assistant").last();
  await expect(reply).toContainText("已完成", { timeout: 120000 });
  await expect(reply.locator("p").first()).not.toBeEmpty();
  expect(sent.length).toBeGreaterThanOrEqual(2);
  expect(sent.every(x => x === "bearer")).toBeTruthy();
  mkdirSync("evidence", { recursive: true });
  await page.screenshot({ path: "evidence/week05-student-stream.png", fullPage: true });
  await page.getByRole("button", { name: "退出登录" }).click();
  await expect(page.getByRole("heading", { name: "登录教学空间" })).toBeVisible();
  await signIn(page, "student_a", process.env.SEED_STUDENT_A_PASSWORD!);
  await page.getByRole("button", { name: /解题助手/ }).click();
  await page.getByRole("button", { name: /方程中为什么要保持等式两边/ }).first().click();
  await expect(page.locator(".chat-message.assistant").last()).toContainText("已完成");
});

test("教师可看到智能体及其技能、材料绑定编辑入口", async ({ page }) => {
  await signIn(page, "teacher_a", process.env.SEED_TEACHER_A_PASSWORD!);
  await page.getByRole("button", { name: /解题助手/ }).click();
  await page.getByText("教师：智能体与技能配置").click();
  await page.getByLabel("编辑智能体").selectOption({ label: "数学助手" });
  await expect(page.getByLabel("智能体名称")).toHaveValue("数学助手");
  await expect(page.getByRole("group", { name: "绑定技能" })).toContainText("数学解题引导");
  await expect(page.getByRole("group", { name: "绑定材料" })).toContainText("数学自学笔记.md");
});
