package types

// CloudProvider represents the cloud platform hosting the resource
type CloudProvider string

const (
	// CloudProviderAWS represents Amazon Web Services
	CloudProviderAWS CloudProvider = "AWS"
	// CloudProviderGCP represents Google Cloud Platform
	CloudProviderGCP CloudProvider = "GCP"
	// CloudProviderAzure represents Microsoft Azure
	CloudProviderAzure CloudProvider = "AZURE"
	// CloudProviderUnknown represents an unknown or unspecified cloud provider
	CloudProviderUnknown CloudProvider = "UNKNOWN"
)

// String returns the string representation of the CloudProvider
func (c CloudProvider) String() string {
	return string(c)
}

// IsValid returns true if the CloudProvider is a known value
func (c CloudProvider) IsValid() bool {
	switch c {
	case CloudProviderAWS, CloudProviderGCP, CloudProviderAzure:
		return true
	default:
		return false
	}
}
