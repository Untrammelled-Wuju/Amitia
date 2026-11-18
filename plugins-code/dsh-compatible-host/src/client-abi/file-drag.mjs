function fileName(source) {
  const name = String(source?.name || "").trim().replaceAll("\\", "/").split("/").pop();
  return name || "download";
}

export function createDshFileDragActions() {
  return {
    async prepare(source, signal) {
      if (!source || (source.kind !== "blob" && source.kind !== "url")) {
        throw new Error(`DSH file drag source is invalid: ${JSON.stringify(source)}`);
      }
      if (signal?.aborted) throw signal.reason ?? new DOMException("The operation was aborted", "AbortError");
      const blob = source.kind === "blob"
        ? source.blob
        : (await fetch(source.url, { signal })).blob();
      const resolved = await blob;
      let released = false;
      return {
        start(transfer) {
          if (released || signal?.aborted || !transfer) return false;
          const file = new File([resolved], fileName(source), {
            type: String(source.mediaType || resolved.type || "application/octet-stream"),
          });
          if (typeof transfer.items?.add === "function") transfer.items.add(file);
          else transfer.items?.push?.(file);
          transfer.effectAllowed = "copy";
          return true;
        },
        release() {
          released = true;
        },
      };
    },
    reportFailure() {
      globalThis.dispatchEvent?.(new CustomEvent("amitia:dsh:file-drag-failure", {
        detail: { message: "文件拖出失败" },
      }));
    },
  };
}
