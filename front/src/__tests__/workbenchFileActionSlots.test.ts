import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { describe, expect, it, vi } from "vitest";
import WorkbenchResourceView from "../components/workbench/WorkbenchResourceView.vue";
import { useWorkbenchStore } from "../stores/workbench";

const apiGet = vi.hoisted(() => vi.fn());
const apiPost = vi.hoisted(() => vi.fn());

vi.mock("../composables/useApi", () => ({
  apiClient: {
    get: apiGet,
    post: apiPost,
  },
}));

function mountView(tab: {
  id: string;
  kind: "file" | "git";
  title: string;
  resourceUri: string;
  workspaceUri: string;
  targetDeviceId: string;
  createdAt: number;
  pinned: boolean;
}) {
  const pinia = createPinia();
  setActivePinia(pinia);
  const store = useWorkbenchStore();
  store.setConversation("conversation-a", "role-a");
  return mount(WorkbenchResourceView, {
    props: { tab },
    global: {
      plugins: [pinia],
      stubs: {
        ExtensionSlot: {
          props: ["slotId", "context"],
          template:
            '<div :data-slot-id="slotId" :data-slot-context="JSON.stringify(context)"></div>',
        },
        WorkbenchBrowserView: { template: "<div />" },
        WorkbenchTerminalView: { template: "<div />" },
        WorkbenchTasksView: { template: "<div />" },
        WorkbenchAgentRunsView: { template: "<div />" },
      },
    },
  });
}

describe("workbench file action slots", () => {
  it("mounts file preview actions with exact event coordinates", async () => {
    apiGet.mockResolvedValue({
      data: {
        isText: true,
        contentBase64: "aGk=",
        bytesRead: 2,
        nextOffset: 2,
        hasMore: false,
      },
    });
    const wrapper = mountView({
      id: "file-tab",
      kind: "file",
      title: "notes.txt",
      resourceUri: "/workspace/notes.txt",
      workspaceUri: "/workspace",
      targetDeviceId: "",
      createdAt: 1,
      pinned: false,
    });
    await flushPromises();

    const slot = wrapper.get('[data-slot-id="workbench.file.actions"]');
    expect(JSON.parse(slot.attributes("data-slot-context") || "{}")).toMatchObject({
      conversationId: "conversation-a",
      characterId: "role-a",
      workspaceUri: "/workspace",
      resourceUri: "/workspace/notes.txt",
      filePath: "notes.txt",
      fileName: "notes.txt",
      surface: "preview",
      eventCoordinates: {
        conversationId: "conversation-a",
        characterId: "role-a",
        workspaceUri: "/workspace",
        resourceUri: "/workspace/notes.txt",
      },
    });
    wrapper.unmount();
  });

  it("mounts review actions on every changed file", async () => {
    apiGet.mockResolvedValue({
      data: [{ id: "mount-a", name: "工作区", kind: "local", rootUri: "/workspace", available: true }],
    });
    apiPost.mockImplementation(async (path: string) => {
      if (path === "/api/workspaces/git/status") {
        return {
          data: {
            branch: "develop",
            ahead: 0,
            behind: 0,
            clean: false,
            entries: [
              { uri: "/workspace/a.ts", staging: " ", worktree: "M", conflict: false, untracked: false },
              { uri: "/workspace/b.ts", staging: "?", worktree: "?", conflict: false, untracked: true },
            ],
          },
        };
      }
      if (path === "/api/workspaces/git/log") return { data: { entries: [] } };
      if (path === "/api/workspaces/git/worktrees") return { data: { entries: [] } };
      throw new Error(`unexpected API request: ${path}`);
    });
    const wrapper = mountView({
      id: "git-tab",
      kind: "git",
      title: "Git",
      resourceUri: "",
      workspaceUri: "/workspace",
      targetDeviceId: "",
      createdAt: 2,
      pinned: false,
    });
    await flushPromises();

    const slots = wrapper.findAll('[data-slot-id="workbench.review.file.actions"]');
    expect(slots).toHaveLength(2);
    expect(
      slots.map((slot) =>
        JSON.parse(slot.attributes("data-slot-context") || "{}").resourceUri,
      ),
    ).toEqual(["/workspace/a.ts", "/workspace/b.ts"]);
    expect(
      slots.every(
        (slot) =>
          JSON.parse(slot.attributes("data-slot-context") || "{}").surface ===
          "review",
      ),
    ).toBe(true);
    wrapper.unmount();
  });
});
