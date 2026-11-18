import assert from "node:assert/strict";
import test from "node:test";
import {
  DshClientModuleLoader,
  installDshModuleLoader,
  loadDshClientPlugin,
} from "../src/client-abi/module-loader.mjs";

test("implements the DSH ModuleLoader factory and require contract", () => {
  const loader = new DshClientModuleLoader();
  loader.load({
    id: "@fixture/client",
    factory(require) {
      const react = require("react");
      return {
        value: react.version,
        apply() {},
      };
    },
  });
  assert.equal(loader.instantiate("@fixture/client", { react: { version: "19.0.0" } }).value, "19.0.0");
  assert.throws(
    () => loader.instantiate("@fixture/client", {}),
    /unavailable dependency react/,
  );
});

test("applies a client plugin with slots, locale, effects and events", async () => {
  const loader = new DshClientModuleLoader();
  const calls = [];
  loader.load({
    id: "@fixture/client-ui",
    factory() {
      return {
        inject: ["slots", "locale"],
        apply(ctx) {
          ctx.effect(() => () => calls.push("effect-disposed"));
          ctx.locale.register("fixture", { zh: { title: "标题" } });
          ctx.slots.register({ name: "shell.overlay", id: "fixture" }, () => "ui");
          ctx.on("fixture/ping", (value) => calls.push(`ping:${value}`));
        },
      };
    },
  });
  const runtime = await loadDshClientPlugin({
    loader,
    id: "@fixture/client-ui",
    modules: {},
    slots: {
      register(options, component) {
        calls.push(`slot:${options.name}:${options.id}:${component()}`);
        return () => calls.push("slot-disposed");
      },
    },
    locale: {
      register(namespace, dictionaries) {
        calls.push(`locale:${namespace}:${dictionaries.zh.title}`);
        return () => calls.push("locale-disposed");
      },
    },
  });
  await runtime.eventBus.emit("fixture/ping", "ok");
  await runtime.dispose();
  assert.deepEqual(calls, [
    "locale:fixture:标题",
    "slot:shell.overlay:fixture:ui",
    "ping:ok",
    "effect-disposed",
  ]);
});

test("installs one loader on the target global", () => {
  const target = {};
  const first = installDshModuleLoader(target);
  const second = installDshModuleLoader(target);
  assert.equal(first, second);
  assert.equal(target.__ModuleLoader__, first);
});

test("tracks replacement client records and removes only the current record", () => {
  const loader = new DshClientModuleLoader();
  const first = { id: "@fixture/reload", factory() { return { apply() {} }; } };
  const second = { id: "@fixture/reload", factory() { return { apply() {} }; } };
  loader.load(first);
  assert.equal(loader.record("@fixture/reload"), first);
  loader.load(second);
  assert.equal(loader.record("@fixture/reload"), second);
  loader.remove("@fixture/reload", first);
  assert.equal(loader.record("@fixture/reload"), second);
  loader.remove("@fixture/reload", second);
  assert.equal(loader.record("@fixture/reload"), undefined);
});

test("tracks asynchronous client effect disposers through teardown", async () => {
  const loader = new DshClientModuleLoader();
  const calls = [];
  loader.load({
    id: "@fixture/async-effect",
    factory() {
      return {
        apply(ctx) {
          ctx.effect(async () => () => calls.push("async-disposed"));
        },
      };
    },
  });
  const runtime = await loadDshClientPlugin({
    loader,
    id: "@fixture/async-effect",
    modules: {},
    slots: {},
    locale: {},
  });
  await runtime.dispose();
  assert.deepEqual(calls, ["async-disposed"]);
});

test("provides and removes DSH client services", async () => {
  const loader = new DshClientModuleLoader();
  loader.load({
    id: "@fixture/client-service",
    factory() {
      return {
        apply(ctx) {
          const dispose = ctx.provide("temporary", 1);
          ctx.set("temporary", 2);
          globalThis.__clientServiceValue = ctx.get("temporary");
          dispose();
        },
      };
    },
  });
  const runtime = await loadDshClientPlugin({
    loader,
    id: "@fixture/client-service",
    modules: {},
    slots: {},
    locale: {},
  });
  assert.equal(globalThis.__clientServiceValue, 2);
  assert.equal(runtime.context.get("temporary"), undefined);
  delete globalThis.__clientServiceValue;
  await runtime.dispose();
});

test("resolves package client subpath dependencies from registered modules", () => {
  const loader = new DshClientModuleLoader();
  loader.load({
    id: "@fixture/gateway",
    factory() {
      return { decide: () => "allowed" };
    },
  });
  loader.load({
    id: "@fixture/app/client",
    factory(require) {
      const gateway = require("@fixture/gateway/client");
      return {
        value: gateway.decide(),
        apply() {},
      };
    },
  });
  const modules = {};
  assert.equal(loader.instantiate("@fixture/app/client", modules).value, "allowed");
  assert.equal(loader.exportsFor(modules).get("@fixture/gateway").decide(), "allowed");
  loader.load({
    id: "@fixture/strict/client",
    factory(require) {
      require("@fixture/gateway/types");
      return { apply() {} };
    },
  });
  assert.throws(
    () => loader.instantiate("@fixture/strict/client", {}),
    /unsupported subpath @fixture\/gateway\/types/,
  );
});

test("composes DSH client waterfall listeners with the trailing fallback", async () => {
  const loader = new DshClientModuleLoader();
  loader.load({
    id: "@fixture/waterfall",
    factory() {
      return {
        apply(ctx) {
          ctx.on("value", (input, next) => next(input + 1));
          ctx.on("value", async (input, next) => next(input * 2));
          globalThis.__clientWaterfall = {
            composed: ctx.waterfall("value", 3, (input) => input),
            fallbackOnly: ctx.waterfall("absent", 3, (input) => input + 100),
          };
        },
      };
    },
  });
  const runtime = await loadDshClientPlugin({
    loader,
    id: "@fixture/waterfall",
    modules: {},
    slots: {},
    locale: {},
  });
  assert.equal(await globalThis.__clientWaterfall.composed, 8);
  assert.equal(await globalThis.__clientWaterfall.fallbackOnly, 103);
  assert.throws(() => runtime.context.waterfall("value"), /requires an event name and a fallback/);
  assert.throws(() => runtime.context.waterfall("value", 3, "fallback"), /fallback must be a function/);
  delete globalThis.__clientWaterfall;
  await runtime.dispose();
});
