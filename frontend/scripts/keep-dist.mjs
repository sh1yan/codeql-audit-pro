// Vite 会清空 dist/；留一个占位文件，保证 `go test ./...` 在没有前端构建产物时也能编译
// （main.go 使用 //go:embed all:frontend/dist）。
import { writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const here = dirname(fileURLToPath(import.meta.url));
writeFileSync(join(here, "..", "dist", ".gitkeep"), "");
