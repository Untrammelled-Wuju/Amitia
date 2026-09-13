import type { PluginChannelSpec } from '@amitia/game-plugin-sdk';

type Assert<T extends true> = T;
type ChannelDirection = NonNullable<PluginChannelSpec['direction']>;
type ChannelFrequencyHint = NonNullable<PluginChannelSpec['frequencyHint']>;

type _PluginToHostDirection = Assert<'plugin_to_host' extends ChannelDirection ? true : false>;
type _HostToPluginDirection = Assert<'host_to_plugin' extends ChannelDirection ? true : false>;
type _BidirectionalDirection = Assert<'bidirectional' extends ChannelDirection ? true : false>;
type _LowFrequency = Assert<'low' extends ChannelFrequencyHint ? true : false>;
type _NormalFrequency = Assert<'normal' extends ChannelFrequencyHint ? true : false>;
type _HighFrequency = Assert<'high' extends ChannelFrequencyHint ? true : false>;
type _RealtimeFrequency = Assert<'realtime' extends ChannelFrequencyHint ? true : false>;
