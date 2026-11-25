import assert from "node:assert/strict";
import test from "node:test";
import { JSDOM } from "jsdom";
import { acquireDshIconSprite, renderDshIconSprite } from "../src/client-ui/icon-sprite.mjs";

const symbols = {
  "dsw-icon-archive-outline-regular": '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1"><path d="M2 5h12"></path></svg>',
  "dsw-icon-pin-outline-regular": '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor"><path d="M8 2v9"></path></svg>',
};

function withDom(run) {
  const dom = new JSDOM("<!doctype html><html><head></head><body></body></html>", { url: "http://localhost" });
  const previous = globalThis.document;
  globalThis.document = dom.window.document;
  try {
    return run(dom.window.document);
  } finally {
    globalThis.document = previous;
    dom.window.close();
  }
}

test("converts icon artwork into symbols without size attributes", () => {
  const markup = renderDshIconSprite(symbols);
  assert.match(markup, /^<symbol id="dsw-icon-archive-outline-regular" viewBox="0 0 16 16"/);
  assert.match(markup, /stroke="currentColor"/);
  assert.doesNotMatch(markup, /width="16"/);
  assert.doesNotMatch(markup, /xmlns=/);
  assert.equal(markup.match(/<symbol/g)?.length, 2);
});

test("drops entries that are not self-contained svg artwork", () => {
  const markup = renderDshIconSprite({
    "dsw-icon-valid": '<svg viewBox="0 0 16 16"><path d="M0 0"></path></svg>',
    "dsw-icon-missing-viewbox": '<svg width="16" height="16"><path d="M0 0"></path></svg>',
    "dsw-icon-unclosed": '<svg viewBox="0 0 16 16"><path d="M0 0"></path>',
    "bad id": '<svg viewBox="0 0 16 16"><path d="M0 0"></path></svg>',
    "dsw-icon-null": null,
  });
  assert.equal(markup.match(/<symbol/g)?.length, 1);
});

test("mounts one shared sprite and removes it after the last release", () => {
  withDom((document) => {
    const releaseFirst = acquireDshIconSprite(symbols);
    const container = document.getElementById("dsh-icon-sprite");
    assert.ok(container);
    assert.equal(container.querySelectorAll("symbol").length, 2);
    assert.equal(document.querySelectorAll('[id="dsh-icon-sprite"]').length, 1);

    const releaseSecond = acquireDshIconSprite(symbols);
    assert.equal(document.querySelectorAll('[id="dsh-icon-sprite"]').length, 1);

    releaseFirst();
    assert.ok(document.getElementById("dsh-icon-sprite"));

    releaseSecond();
    assert.equal(document.getElementById("dsh-icon-sprite"), null);
  });
});

test("rebuilds the sprite after it is removed externally", () => {
  withDom((document) => {
    const releaseFirst = acquireDshIconSprite(symbols);
    document.getElementById("dsh-icon-sprite").remove();
    const releaseSecond = acquireDshIconSprite(symbols);
    assert.ok(document.getElementById("dsh-icon-sprite"));
    releaseFirst();
    assert.ok(document.getElementById("dsh-icon-sprite"));
    releaseSecond();
    assert.equal(document.getElementById("dsh-icon-sprite"), null);
  });
});

test("release is idempotent and tolerates missing artwork", () => {
  withDom((document) => {
    const releaseMissing = acquireDshIconSprite(undefined);
    releaseMissing();
    assert.equal(document.getElementById("dsh-icon-sprite"), null);

    const release = acquireDshIconSprite(symbols);
    release();
    release();
    assert.equal(document.getElementById("dsh-icon-sprite"), null);
  });
});
