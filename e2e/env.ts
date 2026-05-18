import { existsSync } from "fs";
import { resolve } from "path";
import { config } from "dotenv";

const envCandidates = [".env.worktree", ".env"];

for (const filename of envCandidates) {
  const path = resolve(process.cwd(), filename);
  if (existsSync(path)) {
    config({ path });
    break;
  }
}

if (!process.env.PLAYWRIGHT_BASE_URL) {
  process.env.PLAYWRIGHT_BASE_URL = `http://localhost:${process.env.FRONTEND_PORT || "3000"}`;
}
