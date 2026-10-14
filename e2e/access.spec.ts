import {
  test,
  expect,
  request,
  type APIRequestContext,
} from "@playwright/test";
import { createHmac } from "node:crypto";
let a: APIRequestContext,
  b: APIRequestContext,
  s: APIRequestContext,
  ca: string,
  cb: string,
  uploaded: string;
async function login(c: APIRequestContext, username: string, key: string) {
  const r = await c.post("/api/auth/login", {
    data: { username, password: process.env[key] },
  });
  expect(r.status()).toBe(200);
  const data = await r.json();
  expect(data.token_type).toBe("Bearer");
  expect(data.expires_in).toBe(1800);
  expect(JSON.stringify(data.user)).not.toContain("password");
  authTokens.set(c, data.access_token);
  return data;
}
const authTokens = new WeakMap<APIRequestContext, string>();
async function client() {
  const raw = await request.newContext({
    baseURL: process.env.APP_ORIGIN,
    extraHTTPHeaders: { Origin: process.env.APP_ORIGIN! },
  });
  const wrapped: APIRequestContext = new Proxy(raw, {
    get(target, key) {
      if (key === "get" || key === "post")
        return (
          url: string,
          options: Parameters<APIRequestContext["get"]>[1] = {},
        ) =>
          target[key](url, {
            ...options,
            headers: {
              ...(authTokens.has(wrapped)
                ? { Authorization: `Bearer ${authTokens.get(wrapped)}` }
                : {}),
              ...options.headers,
            },
          });
      const value = Reflect.get(target, key);
      return typeof value === "function" ? value.bind(target) : value;
    },
  });
  return wrapped;
}
test.beforeAll(async () => {
  a = await client();
  b = await client();
  s = await client();
  ca = (await login(a, "teacher_a", "SEED_TEACHER_A_PASSWORD")).user.classes[0]
    .id;
  cb = (await login(b, "teacher_b", "SEED_TEACHER_B_PASSWORD")).user.classes[0]
    .id;
  await login(s, "student_a", "SEED_STUDENT_A_PASSWORD");
});
test.afterAll(async () => {
  await Promise.all([a?.dispose(), b?.dispose(), s?.dispose()]);
});
test("无 Bearer、Cookie、URL 凭证不能认证", async ({ request: r }) => {
  expect((await r.get("/api/me")).status()).toBe(401);
  expect(
    (
      await r.get("/api/me", {
        headers: { Cookie: "campus_session=oldsession" },
      })
    ).status(),
  ).toBe(401);
  const c = await client();
  await login(c, "student_a", "SEED_STUDENT_A_PASSWORD");
  expect((await r.get("/api/me?token=not-a-credential")).status()).toBe(
    401,
  );
  expect(
    (await c.storageState()).cookies.filter((x) => x.name === "campus_session"),
  ).toHaveLength(0);
  await c.dispose();
  expect((await r.get("/health")).status()).toBe(200);
});
test("错误登录、过期和篡改 JWT", async () => {
  const c = await client();
  expect(
    (
      await c.post("/api/auth/login", {
        data: { username: "teacher_a", password: "wrong" },
      })
    ).status(),
  ).toBe(401);
  const data = await login(c, "teacher_a", "SEED_TEACHER_A_PASSWORD");
  const parts = data.access_token.split(".");
  const body = JSON.parse(Buffer.from(parts[1], "base64url").toString());
  body.exp = Math.floor(Date.now() / 1000) - 10;
  const signed =
    parts[0] + "." + Buffer.from(JSON.stringify(body)).toString("base64url");
  const expired =
    signed +
    "." +
    createHmac("sha256", process.env.JWT_SECRET!)
      .update(signed)
      .digest("base64url");
  for (const value of [expired, data.access_token + "x"])
    expect(
      (
        await c.get("/api/me", {
          headers: { Authorization: "Bearer " + value },
        })
      ).status(),
    ).toBe(401);
  await c.dispose();
});
test("退出立即撤销，学生和跨班拒绝", async () => {
  const c = await client();
  const data = await login(c, "student_a", "SEED_STUDENT_A_PASSWORD");
  expect((await c.get("/api/me")).status()).toBe(200);
  expect((await c.post("/api/auth/logout")).status()).toBe(204);
  expect(
    (
      await c.get("/api/me", {
        headers: { Authorization: "Bearer " + data.access_token },
      })
    ).status(),
  ).toBe(401);
  await c.dispose();
  expect(
    (
      await s.post(`/api/classes/${ca}/materials`, {
        data: { role: "teacher" },
      })
    ).status(),
  ).toBe(403);
  expect((await a.get(`/api/classes/${cb}/materials`)).status()).toBe(404);
  expect((await a.post(`/api/classes/${cb}/materials`)).status()).toBe(404);
  const bm = (await (await b.get(`/api/classes/${cb}/materials`)).json())
    .materials[0].id;
  expect((await a.get(`/api/classes/${ca}/materials/${bm}`)).status()).toBe(
    404,
  );
});
test("上传及 Bearer 下载，匿名/跨班下载拒绝", async ({ request: r }) => {
  const body = "# JWT 验收\n真实原文";
  const response = await a.post(`/api/classes/${ca}/materials`, {
    multipart: {
      file: {
        name: "JWT验收.md",
        mimeType: "text/markdown",
        buffer: Buffer.from(body),
      },
    },
  });
  expect(response.status()).toBe(201);
  uploaded = (await response.json()).id;
  const url = `/api/classes/${ca}/materials/${uploaded}/file`;
  expect(await (await s.get(url)).text()).toBe(body);
  expect((await r.get(url)).status()).toBe(401);
  expect((await b.get(url)).status()).toBe(404);
  expect((await a.get("/uploads/JWT验收.md")).status()).toBe(404);
});
test("文件失败不新增记录，写操作来源校验", async () => {
  const url = `/api/classes/${ca}/materials`;
  const count = (await (await a.get(url)).json()).materials.length;
  expect(
    (
      await a.post(url, { headers: { Origin: "https://invalid.example" } })
    ).status(),
  ).toBe(403);
  for (const [name, buffer, status] of [
    ["empty.txt", Buffer.from(" "), 422],
    ["binary.txt", Buffer.from([255]), 415],
    ["a.pdf", Buffer.from("pdf"), 415],
    ["large.txt", Buffer.alloc(10 * 1024 * 1024 + 1, 65), 413],
  ] as const) {
    expect(
      (
        await a.post(url, {
          multipart: {
            file: { name, mimeType: "application/octet-stream", buffer },
          },
        })
      ).status(),
    ).toBe(status);
  }
  expect((await (await a.get(url)).json()).materials.length).toBe(count);
});
test("浏览器 Bearer、下载、刷新退出且没有 me 轮询", async ({ page }) => {
  const calls: string[] = [];
  page.on("request", (r) => calls.push(r.url()));
  await page.goto("/");
  await expect(page).toHaveURL(/login/);
  async function signIn() {
    await page.getByLabel("账号", { exact: true }).fill("teacher_a");
    await page
      .getByLabel("密码", { exact: true })
      .fill(process.env.SEED_TEACHER_A_PASSWORD!);
    await page.getByRole("button", { name: "登录 →" }).click();
    await expect(
      page.getByRole("heading", { name: "班级知识库" }),
    ).toBeVisible();
  }
  const materialRequest = page.waitForRequest((r) =>
    r.url().endsWith("/materials"),
  );
  await signIn();
  expect((await materialRequest).headers()["authorization"]).toMatch(
    /^Bearer /,
  );
  expect(
    await page.evaluate(() => localStorage.length + sessionStorage.length),
  ).toBe(0);
  await page
    .getByRole("button", { name: /JWT验收.md/ })
    .first()
    .click();
  const download = page.waitForEvent("download");
  await page.getByRole("button", { name: "下载原文件 ↓" }).click();
  expect((await download).suggestedFilename()).toBe("JWT验收.md");
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "登录教学空间" }),
  ).toBeVisible();
  await signIn();
  await page.clock.install();
  await page.clock.fastForward(11000);
  expect(calls.filter((u) => u.endsWith("/api/me"))).toHaveLength(0);
  await page.getByRole("button", { name: "退出登录" }).click();
  await expect(
    page.getByRole("heading", { name: "登录教学空间" }),
  ).toBeVisible();
});
test("退出后延迟材料响应不回填", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("账号", { exact: true }).fill("teacher_a");
  await page
    .getByLabel("密码", { exact: true })
    .fill(process.env.SEED_TEACHER_A_PASSWORD!);
  await page.getByRole("button", { name: "登录 →" }).click();
  await expect(
    page.getByRole("button", { name: /JWT验收.md/ }).first(),
  ).toBeVisible();
  let release!: () => void;
  const gate = new Promise<void>((r) => (release = r));
  await page.route("**/materials", async (route) => {
    const response = await route.fetch();
    await gate;
    await route.fulfill({ response }).catch(() => {});
  });
  await page.getByRole("button", { name: "↻ 刷新列表" }).click();
  await page.getByRole("button", { name: "退出登录" }).click();
  release();
  await expect(
    page.getByRole("heading", { name: "登录教学空间" }),
  ).toBeVisible();
  await expect(page.locator(".material")).toHaveCount(0);
});
