import * as React from "react";
import { createRoot } from "react-dom/client";

export const REACT_ADAPTER_ID = "amitia-dsh-react";

export function adaptDshReactProps(props) {
  const source = props && typeof props === "object" ? props : {};
  const adapted = { ...source };
  for (const [name, value] of Object.entries(source)) {
    if (!name.startsWith("use") || typeof value !== "function") continue;
    adapted[name] = (...args) => {
      const result = value(...args);
      return result && typeof result === "object" && Object.prototype.hasOwnProperty.call(result, "value")
        ? result.value
        : result;
    };
  }
  return adapted;
}

export function registerReactAdapter(registry, adapterId = REACT_ADAPTER_ID) {
  if (!registry) throw new Error("Amitia foreign component registry is unavailable");
  if (registry.get(adapterId)) return;
  registry.register({
    id: adapterId,
    mount(input) {
      const component = input.props.component;
      if (typeof component !== "function" && typeof component !== "object") {
        throw new Error("DSH React component is unavailable");
      }
      const root = createRoot(input.element);
      const render = (nextInput) => {
        const componentProps = adaptDshReactProps(nextInput.props.componentProps ?? {});
        root.render(React.createElement(component, componentProps));
      };
      render(input);
      return {
        async update(nextInput) {
          render(nextInput);
        },
        async unmount() {
          root.unmount();
        },
      };
    },
  });
}
