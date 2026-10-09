import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";

const state = vi.hoisted(() => ({ base: "http://core.example", get: vi.fn(), post: vi.fn(), delete: vi.fn() }));
vi.mock("@/composables/useApi", () => ({ apiClient: { get: state.get, post: state.post, delete: state.delete } }));
vi.mock("@/runtime/runtime-adapter", () => ({ getApiBaseURLForPath: async () => state.base, getBackendAuthHeaders: async () => ({}) }));
vi.mock("element-plus", () => ({ ElMessage: { success: vi.fn(), warning: vi.fn(), error: vi.fn() } }));

import { resolveConversationMediaUrl } from "@/conversation/rendering/media";
import { buildHtmlPreviewDocument } from "@/conversation/rendering/preview/htmlDocument";
import { renderMarkdownSegment } from "@/conversation/rendering/markdown/markdownEngine";
import AmitiaArtifactBlock from "@/conversation/rendering/blocks/AmitiaArtifactBlock.vue";
import AmitiaImageBlock from "@/conversation/rendering/blocks/AmitiaImageBlock.vue";
import MarkdownContent from "@/conversation/rendering/markdown/MarkdownContent.vue";
import AmitiaFileBlock from "@/conversation/rendering/blocks/AmitiaFileBlock.vue";

beforeEach(() => {
  URL.createObjectURL ||= () => "blob:test";
  URL.revokeObjectURL ||= () => {};
  state.base = "http://core.example";
  state.get.mockReset();
  state.post.mockReset().mockResolvedValue({ data: { previewId: "preview", url: "/media/artifact-previews/preview/ticket" } });
  state.delete.mockResolvedValue({ data: { ok: true } });
  window.dispatchEvent(new Event("amitia:runtime-connection-changed"));
  vi.unstubAllGlobals();
});

describe("conversation file loading", () => {
  it("resolves resource and content addresses through the owning Core", async () => {
    state.get.mockResolvedValue({ data: { url: "/media/artifacts/a/ticket", expiresAt: new Date(Date.now() + 3600000).toISOString() } });
    expect(await resolveConversationMediaUrl("amitia://artifacts/a")).toBe("http://core.example/media/artifacts/a/ticket");
    expect(await resolveConversationMediaUrl("http://core.example/api/artifacts/v1/a/content")).toBe("http://core.example/media/artifacts/a/ticket");
    expect(state.get).toHaveBeenCalledTimes(1);
  });

  it("does not reuse tickets after a Core change", async () => {
    state.get.mockResolvedValue({ data: { url: "/media/artifacts/a/ticket" } });
    await resolveConversationMediaUrl("amitia://artifacts/a");
    state.base = "https://other.example";
    expect(await resolveConversationMediaUrl("amitia://artifacts/a")).toBe("https://other.example/media/artifacts/a/ticket");
    expect(state.get).toHaveBeenCalledTimes(2);
  });

  it("uses authenticated content for provider relay media", async () => {
    state.base = "http://localhost/internal/device-mesh/provider";
    state.get.mockResolvedValue({ data: new Blob(["image"], { type: "image/png" }) });
    const create = vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:preview");
    expect(await resolveConversationMediaUrl("amitia://artifacts/a")).toBe("blob:preview");
    expect(state.get).toHaveBeenCalledWith("/api/artifacts/v1/a/content", { responseType: "blob" });
    create.mockRestore();
  });

  it("renders complete HTML with a base and network resources inside a restricted document", async () => {
    const html = await buildHtmlPreviewDocument('<!doctype html><html><head><title>Page</title><link href="style.css" rel="stylesheet"></head><body><img src="photo.png"><script>window.ready=1</script></body></html>', "https://site.example/report/index.html");
    const document = new DOMParser().parseFromString(html, "text/html");
    expect(document.querySelectorAll("html")).toHaveLength(1);
    expect(document.querySelector("base")?.href).toBe("https://site.example/report/index.html");
    expect(document.querySelector("title")?.textContent).toBe("Page");
    expect(document.querySelector('[http-equiv="Content-Security-Policy"]')?.getAttribute("content")).toContain("connect-src 'none'");
    expect(html).toContain("img-src https: http:");
  });

  it("exchanges attachment references embedded in HTML", async () => {
    state.get.mockResolvedValue({ data: { url: "/media/artifacts/a/ticket" } });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, blob: async () => new Blob(["image"], { type: "image/png" }) }));
    const html = await buildHtmlPreviewDocument('<img src="amitia://artifacts/a">');
    expect(html).toContain('src="data:image/png;base64,');
  });

  it("preserves Markdown attachment addresses until asynchronous resolution", () => {
    const html = renderMarkdownSegment("![picture](amitia://artifacts/a)", []);
    expect(html).toContain('data-amitia-source="amitia://artifacts/a"');
    expect(html).not.toContain('src="amitia:');
  });

  it("shows resolution failures and permits image retry", async () => {
    state.get.mockRejectedValueOnce(new Error("unavailable")).mockResolvedValueOnce({ data: { url: "/media/artifacts/a/ticket" } });
    const image = { id: "a", kind: "image", url: "amitia://artifacts/a", status: "loading", alt: "image" } as any;
    const wrapper = mount(AmitiaImageBlock, { props: { images: [image] } });
    await flushPromises();
    expect(wrapper.find("button.failed").exists()).toBe(true);
    await wrapper.find("button.failed").trigger("click");
    await flushPromises();
    expect(wrapper.find("img").attributes("src")).toContain("amrpRetry=1");
    wrapper.unmount();
  });

  it("loads URL-only HTML artifacts before preview", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, text: async () => "<h1>Loaded file</h1>" }));
    const wrapper = mount(AmitiaArtifactBlock, { props: { block: { id: "a", kind: "artifact", artifactKind: "html", title: "index.html", url: "https://site.example/index.html" } as any } });
    await wrapper.find("button").trigger("click");
    await flushPromises();
    expect(state.post.mock.calls[0][1].html).toContain("Loaded file");
    expect(wrapper.find("iframe").attributes("src")).toBe("http://core.example/media/artifact-previews/preview/ticket");
    expect(wrapper.find("iframe").attributes("sandbox")).toBe("allow-scripts");
    wrapper.unmount();
  });

  it("resolves Markdown images after mount and content updates", async () => {
    state.get.mockImplementation(async (path: string) => ({ data: { url: `/media${path}/ticket` } }));
    const wrapper = mount(MarkdownContent, { attachTo: document.body, props: { source: "![image](amitia://artifacts/a)" } });
    await flushPromises();
    expect(wrapper.find("img").attributes("src")).toContain("/a/media-ticket/ticket");
    await wrapper.setProps({ source: "![image](amitia://artifacts/b)" });
    await new Promise(resolve => setTimeout(resolve, 100));
    await flushPromises();
    expect(wrapper.find("img").attributes("src")).toContain("/b/media-ticket/ticket");
    wrapper.unmount();
  });

  it("offers HTML preview for ordinary file messages", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, text: async () => "<h1>File message</h1>" }));
    const wrapper = mount(AmitiaFileBlock, { props: { block: { id: "file", kind: "file", name: "report.html", url: "https://site.example/report.html", status: "ready" } } });
    await wrapper.find("button").trigger("click");
    await flushPromises();
    expect(state.post.mock.calls[0][1].html).toContain("File message");
    wrapper.unmount();
  });
});
