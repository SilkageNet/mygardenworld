import { execFileSync } from "node:child_process";
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import { join, relative, resolve } from "node:path";
import { ESLint } from "eslint";
import { afterAll, describe, expect, it } from "vitest";

// GHSA-vfj7-8cjw-p6xm has no patched braces release. The version-scoped
// package.json override replaces Next's sole fast-glob use (globSync with
// onlyDirectories) with tinyglobby, removing micromatch/braces entirely.
// Keep this contract test when upgrading Next; never silence the audit.
const localRequire = createRequire(import.meta.url);
const configRequire = createRequire(localRequire.resolve("eslint-config-next"));
const pluginEntry = configRequire.resolve("@next/eslint-plugin-next");
const pluginRequire = createRequire(pluginEntry);
const { getRootDirs } = pluginRequire("./utils/get-root-dirs.js") as {
  getRootDirs: (context: { cwd: string; settings: { next?: { rootDir: string | string[] } } }) => string[];
};
const fixture = mkdtempSync(join(tmpdir(), "garden-eslint-glob-"));
for (const name of ["one", "two"]) {
  const pages = join(fixture, "packages", name, "pages");
  mkdirSync(pages, { recursive: true });
  writeFileSync(join(pages, "index.tsx"), "export default function Page() { return null; }");
  writeFileSync(join(pages, "about.tsx"), "export default function About() { return null; }");
}
writeFileSync(join(fixture, "packages", "not-a-directory.txt"), "fixture");
afterAll(() => rmSync(fixture, { recursive: true, force: true }));

describe("Next ESLint glob security replacement", () => {
  it("loads the actual replacement package, not an ignored vulnerable dependency", () => {
    const metadata = JSON.parse(readFileSync(pluginRequire.resolve("fast-glob/package.json"), "utf8"));
    expect(metadata.name).toBe("tinyglobby");
    const lockfile = readFileSync(new URL("../../pnpm-lock.yaml", import.meta.url), "utf8");
    expect(lockfile).not.toMatch(/^\s+(?:braces|micromatch|fast-glob)@/m);
  });

  it.each([
    ["default cwd", undefined, [fixture]],
    ["literal", join(fixture, "packages", "one"), [join(fixture, "packages", "one")]],
    ["relative literal", relative(process.cwd(), join(fixture, "packages", "one")), [join(fixture, "packages", "one")]],
    ["wildcard directories", join(fixture, "packages", "*"), [join(fixture, "packages", "one"), join(fixture, "packages", "two")]],
    ["brace alternatives", join(fixture, "packages", "{one,two}"), [join(fixture, "packages", "one"), join(fixture, "packages", "two")]],
    ["root array", [join(fixture, "packages", "one"), join(fixture, "packages", "two")], [join(fixture, "packages", "one"), join(fixture, "packages", "two")]],
    ["no match", join(fixture, "missing", "*"), []],
    ["files excluded", join(fixture, "packages", "not-a-directory.txt"), []],
  ] as const)("preserves %s matching", (_name, rootDir, expected) => {
    const settings = rootDir === undefined ? {} : { next: { rootDir: typeof rootDir === "string" ? rootDir : [...rootDir] } };
    expect(getRootDirs({ cwd: fixture, settings }).map((dir) => resolve(dir)).sort()).toEqual([...expected].sort());
  });

  it("keeps the Next internal-link lint rule active", async () => {
    const root = join(fixture, "packages", "one");
    const eslint = new ESLint({
      cwd: root,
      overrideConfigFile: true,
      overrideConfig: [{
        files: ["**/*.jsx"],
        languageOptions: { parserOptions: { ecmaFeatures: { jsx: true } } },
        settings: { next: { rootDir: join(fixture, "packages", "*") } },
        plugins: { "@next/next": pluginRequire(pluginEntry) },
        rules: { "@next/next/no-html-link-for-pages": "error" },
      }],
    });
    const [result] = await eslint.lintText('export default function Page() { return <a href="/about">About</a>; }', { filePath: join(root, "pages", "index.jsx") });
    expect(result.messages.some((message) => message.ruleId === "@next/next/no-html-link-for-pages")).toBe(true);
  });

  it("handles deeply nested brace input without stack exhaustion", () => {
    // Isolate the regression input and bound its CPU time. It is under the
    // vulnerable braces parser's 10,000-character limit (issue #70).
    const output = execFileSync(process.execPath, ["-e", `
      const { globSync } = require(process.argv[1]);
      const pattern = "{".repeat(4000) + "a,b" + "}".repeat(4000);
      const matches = globSync(pattern, { cwd: process.argv[2], onlyDirectories: true });
      if (matches.length !== 0) process.exit(1);
      process.stdout.write("ok");
    `, pluginRequire.resolve("fast-glob"), fixture], { timeout: 5000, encoding: "utf8" });
    expect(output).toBe("ok");
  });
});
