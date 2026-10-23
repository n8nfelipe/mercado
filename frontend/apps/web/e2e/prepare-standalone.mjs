import { cp, mkdir } from "node:fs/promises";

const web = new URL("../", import.meta.url);
const standalone = new URL(".next/standalone/frontend/apps/web/", web);

await mkdir(new URL(".next/", standalone), { recursive: true });
await cp(new URL(".next/static", web), new URL(".next/static", standalone), {
  recursive: true,
});
await cp(new URL("public", web), new URL("public", standalone), {
  recursive: true,
});
