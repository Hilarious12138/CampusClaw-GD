import { defineConfig } from "@playwright/test";
import { readFileSync } from "node:fs";
for (const line of readFileSync(".env", "utf8").split("\n")) {
  const i = line.indexOf("=");
  if (i > 0 && !line.startsWith("#"))
    process.env[line.slice(0, i)] = line.slice(i + 1);
}
export default defineConfig({
  testDir: "./e2e",
  workers: 1,
  use: { baseURL: process.env.APP_ORIGIN, channel: "chrome", headless: true },
  reporter: "list",
});
