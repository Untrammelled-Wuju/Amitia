import { describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { buildOwnedMemoryGraph } from "../runtime/owned-memory-graph";
import OwnedMemoryGraph from "../components/OwnedMemoryGraph.vue";

vi.mock("vue-echarts", () => ({ default: { name: "VChart", props: ["option"], template: '<div data-graph-canvas />' } }));
const resource = (id: string, content: any, ownerId = "core") => ({ id, kind: "graph", ownerId, roleId: "role", body: { key: "饮品偏好", content } });

describe("当前角色记忆图谱", () => {
  it("将已保存的事实关系转成节点和带名称的连线并去重", () => {
    const rows = [resource("one", { subject: "用户", relation: "喜欢", target: "茶" }), resource("two", { subject: "用户", relation: "喜欢", target: "茶" })];
    const graph = buildOwnedMemoryGraph(rows);
    expect(graph.nodes.map((node) => node.name)).toEqual(["用户", "茶"]);
    expect(graph.links).toHaveLength(1);
    expect(graph.links[0]).toEqual({ source: graph.nodes[0].id, target: graph.nodes[1].id, name: "喜欢" });
  });
  it("兼容节点和关系数组，同名节点不跨数据所有者合并", () => {
    const content = { nodes: [{ id: "a", label: "用户" }, { id: "b", label: "茶" }], edges: [{ source: "a", target: "b", label: "喜欢" }] };
    const graph = buildOwnedMemoryGraph([resource("one", content), resource("two", content, "other")]);
    expect(graph.nodes).toHaveLength(4);
    expect(graph.links).toHaveLength(2);
    expect(graph.links[0].source).not.toBe(graph.links[1].source);
  });
  it("忽略删除及其他类型数据，未知关系格式保留独立记录而不编造连线", () => {
    const graph = buildOwnedMemoryGraph([{ ...resource("deleted", {}), deleted: true }, { ...resource("memory", {}), kind: "memory" }, resource("unknown", { detail: "事实描述" })]);
    expect(graph.nodes.map((node) => node.name)).toEqual(["饮品偏好"]);
    expect(graph.links).toEqual([]);
  });
  it("渲染关系画布，搜索保留相邻节点，切换数据后清除旧图谱", async () => {
    const wrapper = mount(OwnedMemoryGraph, {
      props: { resources: [resource("one", { subject: "用户", relation: "喜欢", target: "茶" }), resource("two", { subject: "小王", relation: "居住", target: "上海" })], loading: false, error: "" },
      global: { stubs: {
        ElInput: { props: ["modelValue"], emits: ["update:modelValue"], template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
        ElButton: { template: "<button><slot /></button>" },
        ElEmpty: { props: ["description"], template: "<div>{{ description }}</div>" },
      } },
    });
    const chart = () => wrapper.findComponent({ name: "VChart" });
    expect(chart().props("option").series[0].data).toHaveLength(4);
    expect(chart().props("option").series[0]).toMatchObject({ type: "graph", roam: true, draggable: true });
    await wrapper.find("input").setValue("茶");
    expect(chart().props("option").series[0].data.map((node: any) => node.name)).toEqual(["用户", "茶"]);
    expect(chart().props("option").series[0].links).toHaveLength(1);
    await wrapper.setProps({ resources: [] });
    expect(chart().exists()).toBe(false);
    expect(wrapper.text()).toContain("没有匹配的图谱节点");
    await wrapper.find("input").setValue("");
    expect(wrapper.text()).toContain("暂无图谱关系");
    wrapper.unmount();
  });
});
