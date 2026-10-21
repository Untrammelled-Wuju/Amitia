import { beforeEach, describe, expect, it, vi } from "vitest";
import importDialogSource from "../views/character-config/components/ImportPackDialog.vue?raw";

const apiMocks = vi.hoisted(() => ({
  get: vi.fn(),
  postUpload: vi.fn(),
}));

vi.mock("../composables/useApi", () => ({
  apiClient: {},
  useApi: () => ({
    get: apiMocks.get,
    postUpload: apiMocks.postUpload,
  }),
}));

vi.mock("element-plus", () => ({
  ElMessage: {
    success: vi.fn(),
    warning: vi.fn(),
    error: vi.fn(),
  },
}));

import { useCharacterImportExport } from "../views/character-config/composables/useCharacterImportExport";

describe("角色包导入弹窗", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("只提供酒馆角色卡 JSON 示例并保留拖放导入", () => {
    expect(importDialogSource).toContain('key: "tavern"');
    expect(importDialogSource).toContain('"creatorcomment"');
    expect(importDialogSource).not.toContain('"spec": "chara_card_v2"');
    expect(importDialogSource).not.toContain('"spec": "chara_card_v3"');
    expect(importDialogSource).not.toContain('"first_mes"');
    expect(importDialogSource).toContain("拖入角色卡文件");
    expect(importDialogSource).toContain('@drop.prevent="onFileDrop"');
    expect(importDialogSource).toContain('ref="fileInput"');
  });

  it("将预览接口响应归一化后交给弹窗展示", async () => {
    apiMocks.postUpload.mockResolvedValue({
      preview: {
        format: "tavern_json",
        name: "阿澈",
        risks: [],
      },
      format: "tavern_json",
      sourceHash: "source-hash",
      importId: "import-id",
    });

    const state = useCharacterImportExport();
    state.setSelectedFile(
      new File(['{"name":"阿澈","description":"旧书店店主"}'], "tavern.json", {
        type: "application/json",
      }),
    );

    await state.previewImport();

    expect(apiMocks.postUpload).toHaveBeenCalledWith(
      "/api/characters/import-card/preview",
      expect.any(File),
    );
    expect(state.importPreview.value).toEqual({
      format: "tavern_json",
      name: "阿澈",
      risks: [],
      sourceHash: "source-hash",
    });
  });

  it("确认导入携带预览生成的幂等导入编号", async () => {
    apiMocks.postUpload
      .mockResolvedValueOnce({
        preview: { format: "v3_charx", name: "阿澈", lorebookEntryCount: 2, risks: [] },
        format: "v3_charx",
        sourceHash: "source-hash",
        importId: "import-123",
        roleAuthority: "a".repeat(64),
      })
      .mockResolvedValueOnce({
        importId: "import-123",
        characterId: "character-1",
        lorebookEntryCount: 2,
        worldbookImportedCount: 2,
      });
    apiMocks.get.mockResolvedValue([]);

    const state = useCharacterImportExport();
    const file = new File(["charx"], "character.charx");
    state.setSelectedFile(file);
    await state.previewImport();
    state.importConfirmText.value = "确认导入";
    await state.confirmImport();

    expect(apiMocks.postUpload).toHaveBeenLastCalledWith(
      "/api/characters/import-card/confirm",
      file,
      "card",
      expect.anything(),
      { importId: "import-123" },
    );
  });
});
