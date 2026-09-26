// Runs `dbx-plugin dev` with a patched copy of the CLI's dev runtime.
// ponytail: plugin-cli 0.1.9 writes go.work with a fixed "go 1.22", which fails for modules needing newer Go.
// The copy uses the version from backend/go.mod instead; delete this script once the CLI is fixed.
import { spawn } from "node:child_process";
import { cpSync, readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const root = new URL("../", import.meta.url);
const runtimeDir = new URL(".dbx-dev/runtime/", root);
cpSync(new URL("node_modules/@dbx-app/plugin-cli/dev-runtime/", root), runtimeDir, { recursive: true });

const goVersion = readFileSync(new URL("backend/go.mod", root), "utf8").match(/^go (\S+)/m)[1];
const runtime = new URL("runtime.mjs", runtimeDir);
writeFileSync(runtime, readFileSync(runtime, "utf8").replace("`go 1.22\n", `\`go ${goVersion}\n`));

const args = ["dev", "--path", ".", "--port", "5190", ...process.argv.slice(2)];
spawn("dbx-plugin", args, {
  cwd: fileURLToPath(root),
  stdio: "inherit",
  env: { ...process.env, DBX_PLUGIN_DEV_RUNTIME: fileURLToPath(runtime) },
}).on("exit", (code) => process.exit(code ?? 1));
