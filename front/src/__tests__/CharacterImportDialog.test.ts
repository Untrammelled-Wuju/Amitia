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
});
