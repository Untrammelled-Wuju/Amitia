import assert from "node:assert/strict";
import test from "node:test";
import { declaredInjectRange } from "../src/runtime/package-manager.mjs";

test("客户端注入依赖按插件声明的范围安装，而不是插件自身版本", () => {
  const manifest = {
    name: "dsh-pet-plugin",
    version: "1.28.0",
    peerDependencies: { "@deepseek-ai/dsh-client-ui-layout": "^0.1.1-rc.1" },
    dependencies: { "@deepseek-ai/dsh-client-ui-conversation": "~0.2.2-rc.2" },
  };
  assert.equal(declaredInjectRange(manifest, "@deepseek-ai/dsh-client-ui-layout"), "@deepseek-ai/dsh-client-ui-layout@^0.1.1-rc.1");
  assert.equal(declaredInjectRange(manifest, "@deepseek-ai/dsh-client-ui-conversation"), "@deepseek-ai/dsh-client-ui-conversation@~0.2.2-rc.2");
  assert.equal(declaredInjectRange(manifest, "@deepseek-ai/dsh-client-runtime"), "@deepseek-ai/dsh-client-runtime@latest");
});
