import {
  PROTOCOL_VERSION,
  ChannelDescriptor,
  PluginSchema,
  ServiceDescriptor,
} from './protocol';
import { validateChannel, validateCapability, validateServices } from './validation';

export interface Descriptor {
  id: string;
  name?: string;
  version?: string;
  protocolVersion: string;
  services: ServiceDescriptor[];
  channels: ChannelDescriptor[];
  capabilities: string[];
  metadata?: Record<string, unknown>;
}

class DescriptorBuilder {
  private descriptor: Descriptor;

  constructor(id: string, name: string, version: string) {
    this.descriptor = {
      id,
      name,
      version,
      protocolVersion: PROTOCOL_VERSION,
      services: [],
      channels: [],
      capabilities: [],
      metadata: {},
    };
  }

  withService(service: ServiceDescriptor): DescriptorBuilder {
    this.descriptor.services.push(service);
    return this;
  }

  withChannel(channel: ChannelDescriptor): DescriptorBuilder {
    this.descriptor.channels.push(channel);
    return this;
  }

  withCapability(capability: string): DescriptorBuilder {
    this.descriptor.capabilities.push(capability);
    return this;
  }

  withMetadata(key: string, value: unknown): DescriptorBuilder {
    if (!this.descriptor.metadata) {
      this.descriptor.metadata = {};
    }
    this.descriptor.metadata[key] = value;
    return this;
  }

  build(): Descriptor {
    this.validate();
    return this.descriptor;
  }

  private validate(): void {
    const errors: string[] = [];

    if (!this.descriptor.id) {
      errors.push('descriptor id must not be empty');
    }
    if (!this.descriptor.protocolVersion || this.descriptor.protocolVersion !== PROTOCOL_VERSION) {
      errors.push(`invalid protocol version: ${this.descriptor.protocolVersion}`);
    }

    const serviceErrors = validateServices(this.descriptor.services);
    errors.push(...serviceErrors);

    const channelErrors = this.descriptor.channels.map(validateChannel).flat();
    errors.push(...channelErrors);

    const capErrors = validateCapability(this.descriptor.capabilities);
    errors.push(...capErrors);

    if (errors.length > 0) {
      throw new Error(`descriptor validation failed: ${errors.join('; ')}`);
    }
  }
}

export function createPluginDescriptor(
  id: string,
  name: string,
  version: string,
): DescriptorBuilder {
  return new DescriptorBuilder(id, name, version);
}
