import { transformWithOxc } from "vite-plus";
import { createRequire } from "node:module";
import path from "node:path";
const require = createRequire(import.meta.url);
const fromVite = createRequire(require.resolve("vite/package.json"));
const helpers = path.dirname(fromVite.resolve("@oxc-project/runtime/package.json"));
// The adapter lowers async-generator syntax. Its dev environment leaves bits-ui
// class fields intact, although Goja's parser rejects the emitted initializer.
// Keep this adjustment in the engine environment; browser modules are untouched.
export function engineSyntax() {
  return {
    name: "gimbal-engine-syntax",
    apply: "serve",
    enforce: "post",
    async transform(code, id) {
      if (this.environment.name !== "goja" || !id.includes("/bits-ui/")) return;
      const result = await transformWithOxc(code, id, { target: "es2017" });
      result.code = result.code.replace(
        /(['"`])@oxc-project\/runtime\/helpers\/([A-Za-z0-9_$]+)\1/g,
        (_, quote, name) => quote + path.join(helpers, "src/helpers/esm", name + ".js") + quote,
      );
      return result;
    },
  };
}
