<script lang="ts">
import {
  defineComponent,
  Fragment,
  h,
  type PropType,
  type VNode,
} from "vue";
import type { UnifiedSlotItem } from "@/ui-runtime/slotLedger";
import ExtensionContributionRenderer from "./ExtensionContributionRenderer.vue";
import ClientSlotContributionHost from "./ClientSlotContributionHost.vue";
import ExtensionErrorBoundary from "./ExtensionErrorBoundary.vue";

export default defineComponent({
  name: "HocSlotLayers",
  props: {
    items: {
      type: Array as PropType<readonly UnifiedSlotItem[]>,
      required: true,
    },
    slotId: {
      type: String,
      required: true,
    },
    context: {
      type: Object as PropType<Record<string, unknown>>,
      required: true,
    },
    hookContext: {
      type: null as unknown as PropType<unknown>,
      default: undefined,
    },
  },
  setup(props, { slots }) {
    const renderLayer = (index: number): VNode => {
      const item = props.items[index]!;
      const children = index + 1 < props.items.length
        ? renderLayer(index + 1)
        : h(Fragment, slots.default?.() ?? []);
      const contribution = item.source === "client"
        ? h(ClientSlotContributionHost, {
            key: item.key,
            contribution: item.client,
            context: props.context,
            matched: item.matched,
            hookContext: props.hookContext,
            hocChildren: children,
          })
        : h("div", {
            key: item.key,
            class: "extension-slot__hoc-server-layer",
            "data-contribution-id": item.server.contributionId,
            "data-extension-id": item.server.extensionId,
          }, [
            h(ExtensionContributionRenderer, {
              contribution: item.server,
              context: props.context,
              slotId: props.slotId,
            }),
            children,
          ]);
      return h(ExtensionErrorBoundary, {
        key: `hoc-layer:${item.key}`,
        slotId: props.slotId,
        contributionId: item.contributionId,
        abdicateOnError: false,
      }, {
        default: () => contribution,
      });
    };
    return () => props.items.length > 0
      ? renderLayer(0)
      : h(Fragment, slots.default?.() ?? []);
  },
});
</script>
