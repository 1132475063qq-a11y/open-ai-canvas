import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

test("Dreamina B completion crosses A Runtime, HTTP wait, and the Web parser without querying from A", async () => {
    const packageRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
    const child = spawn(process.execPath, [
        path.resolve(packageRoot, "node_modules/tsx/dist/cli.mjs"),
        "--tsconfig",
        path.resolve(packageRoot, "../web/tsconfig.json"),
        path.resolve(packageRoot, "test/fixtures/dreamina-cross-runtime-web-fixture.ts"),
    ], {
        cwd: packageRoot,
        stdio: ["ignore", "pipe", "pipe"],
        windowsHide: true,
    });
    const stdout: Buffer[] = [];
    const stderr: Buffer[] = [];
    child.stdout.on("data", (chunk: Buffer) => stdout.push(chunk));
    child.stderr.on("data", (chunk: Buffer) => stderr.push(chunk));
    const exitCode = await new Promise<number | null>((resolve, reject) => {
        child.once("error", reject);
        child.once("exit", resolve);
    });
    assert.equal(exitCode, 0, Buffer.concat(stderr).toString("utf8"));
    assert.match(Buffer.concat(stdout).toString("utf8"), /dreamina-cross-runtime-web-ok/);
});
