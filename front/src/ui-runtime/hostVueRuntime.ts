import * as VueRuntime from "vue";

const HOST_VUE_KEY = "__AMITIA_VUE__";
const host = globalThis as typeof globalThis & {
  [HOST_VUE_KEY]?: typeof VueRuntime;
};

if (!host[HOST_VUE_KEY]) {
  Object.defineProperty(host, HOST_VUE_KEY, {
    configurable: false,
    enumerable: false,
    value: VueRuntime,
    writable: false,
  });
}
