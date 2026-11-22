import test from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));
const cliEntry = resolve(__dirname, "../dist/index.js");

test("Prism CLI binary renders top-level help and subcommands", () => {
  const out = execFileSync(process.execPath, [cliEntry, "--help"], { encoding: "utf8" });
  assert.match(out, /prism/i);
  assert.match(out, /agent/i);
  assert.match(out, /credential/i);
  assert.match(out, /gateway/i);
});
