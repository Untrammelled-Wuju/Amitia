package conformance

func StandardSuite() []Case {
	cases := make([]Case, 0)

	cases = append(cases, ProtocolCases()...)
	cases = append(cases, ProtocolVersionCases()...)
	cases = append(cases, RequestResponseCorrelationCases()...)
	cases = append(cases, OpaquePayloadCases()...)
	cases = append(cases, MetadataForwardCompatibilityCases()...)
	cases = append(cases, PluginMethodValidationCases()...)

	cases = append(cases, ServiceSchemaCases()...)
	cases = append(cases, ChannelSchemaCases()...)
	cases = append(cases, CapabilityCases()...)
	cases = append(cases, PluginErrorCases()...)
	cases = append(cases, DescriptorConformanceCases()...)

	cases = append(cases, InvalidCases()...)

	cases = append(cases, SDKConformanceCases()...)

	return cases
}

func ProtocolCoreSuite() []Case {
	cases := make([]Case, 0)
	cases = append(cases, ProtocolCases()...)
	cases = append(cases, ProtocolVersionCases()...)
	cases = append(cases, RequestResponseCorrelationCases()...)
	cases = append(cases, InvalidCases()...)
	return cases
}

func SchemaSuite() []Case {
	cases := make([]Case, 0)
	cases = append(cases, ServiceSchemaCases()...)
	cases = append(cases, ChannelSchemaCases()...)
	cases = append(cases, CapabilityCases()...)
	cases = append(cases, PluginErrorCases()...)
	cases = append(cases, DescriptorConformanceCases()...)
	return cases
}

func SDKSuite() []Case {
	cases := make([]Case, 0)
	cases = append(cases, SDKConformanceCases()...)
	return cases
}

func CrossLanguageSuite() []Case {
	cases := make([]Case, 0)
	cases = append(cases, OpaquePayloadCases()...)
	cases = append(cases, MetadataForwardCompatibilityCases()...)
	return cases
}
