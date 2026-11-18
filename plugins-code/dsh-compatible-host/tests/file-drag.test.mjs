import assert from "node:assert/strict";
import test from "node:test";
import { createDshFileDragActions } from "../src/client-abi/file-drag.mjs";

test("prepares browser file drag transfers", async () => {
  const actions = createDshFileDragActions();
  const prepared = await actions.prepare({
    kind: "blob",
    blob: new Blob(["image"], { type: "image/png" }),
    name: "preview.png",
    mediaType: "image/png",
  }, new AbortController().signal);
  const transfer = {
    items: {
      files: [],
      add(file) {
        this.files.push(file);
      },
    },
  };
  assert.equal(prepared.start(transfer, {}), true);
  assert.equal(transfer.items.files[0].name, "preview.png");
  assert.equal(transfer.items.files[0].type, "image/png");
  assert.equal(transfer.effectAllowed, "copy");
  prepared.release();
  assert.equal(prepared.start(transfer, {}), false);
});
