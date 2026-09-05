import { execFileSync } from "node:child_process";
import { copyFileSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const [packageName, version, architectures] = process.argv.slice(2);
if (!/^(?:@[a-z0-9][a-z0-9._-]*\/)?[a-z0-9][a-z0-9._-]*$/.test(packageName ?? "") || !/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(version ?? "")) throw new Error("invalid exact npm package");
if (!(architectures ?? "").split(",").every((value) => value === "linux-amd64" || value === "linux-arm64")) throw new Error("invalid architecture");

mkdirSync("/tmp/home", { recursive: true });
mkdirSync("/output", { recursive: true });
const spec = `${packageName}@${version}`;
const packed = JSON.parse(execFileSync("npm", ["pack", spec, "--json", "--ignore-scripts"], { cwd: "/work", encoding: "utf8" }));
const packageTarball = join("/work", packed[0].filename);
copyFileSync(packageTarball, "/output/package.tgz");
execFileSync("npm", ["install", "--ignore-scripts", "--omit=dev", "--no-audit", "--no-fund", "--prefix", "/work/bundle", spec], { stdio: "inherit" });
const packagePath = join("/work/bundle/node_modules", packageName);
const manifest = JSON.parse(readFileSync(join(packagePath, "package.json"), "utf8"));
const bins = typeof manifest.bin === "string" ? { [manifest.name]: manifest.bin } : manifest.bin;
if (!bins || typeof bins !== "object") throw new Error("npm package has no bin metadata");
writeFileSync("/output/bins.json", JSON.stringify(bins), { mode: 0o600 });
writeFileSync("/output/integrity.txt", `${packed[0].integrity}\n`, { mode: 0o600 });
execFileSync("tar", ["--sort=name", "--mtime=@0", "--owner=0", "--group=0", "-czf", "/output/bundle.tgz", "-C", "/work/bundle", "."]);
