import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import DevicePairScanner from '../components/DevicePairScanner.vue';

const scanner = vi.hoisted(() => ({ decode: vi.fn(), image: vi.fn() }));
vi.mock('@zxing/browser', () => ({ BrowserQRCodeReader: class {
  decodeFromConstraints(...args: unknown[]) { return scanner.decode(...args); }
  decodeFromImageUrl(...args: unknown[]) { return scanner.image(...args); }
} }));

const options = { props: { modelValue: false }, global: { stubs: {
  'el-dialog': { template: '<div><slot /><slot name="footer" /></div>' },
  'el-alert': { props: ['title'], template: '<p>{{ title }}</p>' },
  'el-button': { template: '<button><slot /></button>' },
} } };

beforeEach(() => { scanner.decode.mockReset(); scanner.image.mockReset(); });
afterEach(() => vi.restoreAllMocks());

describe('device pairing scanner', () => {
  it('accepts the pairing code once and stops the camera', async () => {
    const stop = vi.fn();
    let callback: (result: { getText: () => string }) => void = () => {};
    scanner.decode.mockImplementation((_constraints, _video, cb) => {
      callback = cb;
      return Promise.resolve({ stop });
    });
    const wrapper = mount(DevicePairScanner, options);
    await wrapper.setProps({ modelValue: true });
    await flushPromises();
    callback({ getText: () => 'https://unrelated.invalid' });
    expect(wrapper.emitted('scanned')).toBeUndefined();
    const payload = 'amitia://pair?endpoint=https%3A%2F%2Fcore.invalid&offer=one-time';
    callback({ getText: () => payload });
    callback({ getText: () => payload });
    expect(wrapper.emitted('scanned')).toEqual([[payload]]);
    expect(stop).toHaveBeenCalledOnce();
    wrapper.unmount();
  });

  it('stops a camera that becomes available after the dialog closes', async () => {
    const stop = vi.fn();
    let resolve: (controls: { stop: () => void }) => void = () => {};
    scanner.decode.mockImplementation(() => new Promise((done) => { resolve = done; }));
    const wrapper = mount(DevicePairScanner, options);
    await wrapper.setProps({ modelValue: true });
    await flushPromises();
    await wrapper.setProps({ modelValue: false });
    resolve({ stop });
    await flushPromises();
    expect(stop).toHaveBeenCalledOnce();
    expect(wrapper.emitted('scanned')).toBeUndefined();
    wrapper.unmount();
  });

  it('releases an acquired camera on unmount', async () => {
    const stop = vi.fn();
    scanner.decode.mockResolvedValue({ stop });
    const wrapper = mount(DevicePairScanner, options);
    await wrapper.setProps({ modelValue: true });
    await flushPromises();
    wrapper.unmount();
    expect(stop).toHaveBeenCalledOnce();
  });
});
