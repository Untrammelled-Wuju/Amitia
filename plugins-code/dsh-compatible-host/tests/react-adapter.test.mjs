import assert from "node:assert/strict";
import test from "node:test";
import { JSDOM } from "jsdom";
import * as React from "react";
import { act } from "react";
import { adaptDshReactProps, registerReactAdapter } from "../src/client-abi/react-adapter.mjs";

test("mounts, updates and unmounts DSH React components", async () => {
  const dom = new JSDOM("<div id=\"root\"></div>");
  const previousWindow = globalThis.window;
  const previousDocument = globalThis.document;
  globalThis.window = dom.window;
  globalThis.document = dom.window.document;
  globalThis.IS_REACT_ACT_ENVIRONMENT = true;
  const registry = {
    adapters: new Map(),
    get(id) {
      return this.adapters.get(id);
    },
    register(adapter) {
      this.adapters.set(adapter.id, adapter);
    },
    mount(id, input) {
      return this.adapters.get(id).mount(input);
    },
  };
  registerReactAdapter(registry);
  const Component = ({ label }) => React.createElement("span", { "data-label": label }, label);
  const element = dom.window.document.getElementById("root");
  const context = { slotId: "ui.portal.overlay" };
  let mounted;
  await act(async () => {
    mounted = await registry.mount("amitia-dsh-react", {
      element,
      props: { component: Component, componentProps: { label: "first" } },
      context,
      emit() {},
    });
  });
  assert.equal(element.querySelector("span").textContent, "first");

  await act(async () => {
    await mounted.update({
      element,
      props: { component: Component, componentProps: { label: "second" } },
      context,
      emit() {},
    });
  });
  assert.equal(element.querySelector("span").textContent, "second");

  await act(async () => {
    await mounted.unmount();
  });
  assert.equal(element.textContent, "");
  globalThis.window = previousWindow;
  globalThis.document = previousDocument;
  delete globalThis.IS_REACT_ACT_ENVIRONMENT;
});

test("unwraps Vue selector refs into React-ready context values", () => {
  const adapted = adaptDshReactProps({
    sessionId: "session-1",
    useSession: (selector) => ({ value: selector({ status: "idle" }) }),
    useStore: () => ({ value: { count: 2 } }),
    t: (key) => key,
  });
  assert.equal(adapted.useSession((session) => session.status), "idle");
  assert.deepEqual(adapted.useStore(), { count: 2 });
  assert.equal(adapted.t("title"), "title");
});
