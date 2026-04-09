package endoflife

import (
	"context"
	"testing"
	"time"

	"github.com/block/Version-Guard/pkg/types"
)

func TestProvider_GetVersionLifecycle_EKS(t *testing.T) {
	// Mock client with test data (using dates relative to 2026-04-08)
	mockClient := &MockClient{
		GetProductCyclesFunc: func(ctx context.Context, product string) ([]*ProductCycle, error) {
			if product != "amazon-eks" {
				t.Errorf("Expected product amazon-eks, got %s", product)
			}
			return []*ProductCycle{
				{
					// Current version - still in standard support
					Cycle:           "1.35",
					ReleaseDate:     "2025-11-19",
					Support:         "2027-12-19", // Future
					EOL:             "2029-05-19", // Far future
					ExtendedSupport: true,
				},
				{
					// Extended support version - past standard, before EOL
					Cycle:           "1.32",
					ReleaseDate:     "2024-05-29",
					Support:         "2025-06-29", // Past
					EOL:             "2027-11-29", // Future
					ExtendedSupport: true,
				},
				{
					// EOL version - past all support dates
					Cycle:           "1.28",
					ReleaseDate:     "2023-09-26",
					Support:         "2024-10-26", // Past
					EOL:             "2026-03-26", // Past (before 2026-04-08)
					ExtendedSupport: true,
				},
			}, nil
		},
	}

	provider := NewProvider(mockClient, 1*time.Hour)

	tests := []struct {
		name           string
		engine         string
		version        string
		wantVersion    string
		wantSupported  bool
		wantDeprecated bool
		wantEOL        bool
	}{
		{
			name:           "current version 1.35",
			engine:         "kubernetes",
			version:        "1.35",
			wantVersion:    "k8s-1.35",
			wantSupported:  true,
			wantDeprecated: false,
			wantEOL:        false,
		},
		{
			name:           "version with k8s- prefix",
			engine:         "kubernetes",
			version:        "k8s-1.35",
			wantVersion:    "k8s-1.35",
			wantSupported:  true,
			wantDeprecated: false,
			wantEOL:        false,
		},
		{
			name:           "eks engine variant",
			engine:         "eks",
			version:        "1.35",
			wantVersion:    "k8s-1.35",
			wantSupported:  true,
			wantDeprecated: false,
			wantEOL:        false,
		},
		{
			name:           "extended support version 1.32",
			engine:         "kubernetes",
			version:        "1.32",
			wantVersion:    "k8s-1.32",
			wantSupported:  true,  // Still in extended support
			wantDeprecated: true,  // Past standard support
			wantEOL:        false, // Not yet EOL
		},
		{
			name:           "eol version 1.28",
			engine:         "kubernetes",
			version:        "1.28",
			wantVersion:    "k8s-1.28",
			wantSupported:  false, // Past all support
			wantDeprecated: true,  // Deprecated
			wantEOL:        true,  // Past EOL date
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lifecycle, err := provider.GetVersionLifecycle(context.Background(), tt.engine, tt.version)
			if err != nil {
				t.Fatalf("GetVersionLifecycle() error = %v", err)
			}

			if lifecycle.Version != tt.wantVersion {
				t.Errorf("Version = %s, want %s", lifecycle.Version, tt.wantVersion)
			}
			if lifecycle.IsSupported != tt.wantSupported {
				t.Errorf("IsSupported = %v, want %v", lifecycle.IsSupported, tt.wantSupported)
			}
			if lifecycle.IsDeprecated != tt.wantDeprecated {
				t.Errorf("IsDeprecated = %v, want %v", lifecycle.IsDeprecated, tt.wantDeprecated)
			}
			if lifecycle.IsEOL != tt.wantEOL {
				t.Errorf("IsEOL = %v, want %v", lifecycle.IsEOL, tt.wantEOL)
			}

			// Verify dates are parsed
			if lifecycle.ReleaseDate == nil {
				t.Error("ReleaseDate should not be nil")
			}
			if lifecycle.DeprecationDate == nil {
				t.Error("DeprecationDate (support end) should not be nil")
			}
			if lifecycle.EOLDate == nil {
				t.Error("EOLDate should not be nil")
			}
		})
	}
}

func TestProvider_ListAllVersions(t *testing.T) {
	mockClient := &MockClient{
		GetProductCyclesFunc: func(ctx context.Context, product string) ([]*ProductCycle, error) {
			return []*ProductCycle{
				{
					Cycle:       "1.35",
					ReleaseDate: "2025-11-19",
					Support:     "2027-12-19",
					EOL:         "2029-05-19",
				},
				{
					Cycle:       "1.34",
					ReleaseDate: "2025-05-29",
					Support:     "2027-06-29",
					EOL:         "2028-11-29",
				},
			}, nil
		},
	}

	provider := NewProvider(mockClient, 1*time.Hour)

	versions, err := provider.ListAllVersions(context.Background(), "kubernetes")
	if err != nil {
		t.Fatalf("ListAllVersions() error = %v", err)
	}

	if len(versions) != 2 {
		t.Errorf("Got %d versions, want 2", len(versions))
	}

	// Verify first version
	if versions[0].Version != "k8s-1.35" {
		t.Errorf("First version = %s, want k8s-1.35", versions[0].Version)
	}
	if versions[0].Engine != "kubernetes" {
		t.Errorf("First version engine = %s, want kubernetes", versions[0].Engine)
	}
	if versions[0].Source != "endoflife-date-api" {
		t.Errorf("Source = %s, want endoflife-date-api", versions[0].Source)
	}
}

func TestProvider_Caching(t *testing.T) {
	callCount := 0
	mockClient := &MockClient{
		GetProductCyclesFunc: func(ctx context.Context, product string) ([]*ProductCycle, error) {
			callCount++
			return []*ProductCycle{
				{
					Cycle:       "1.35",
					ReleaseDate: "2025-11-19",
					Support:     "2027-12-19",
					EOL:         "2029-05-19",
				},
			}, nil
		},
	}

	provider := NewProvider(mockClient, 1*time.Hour)

	// First call - should hit API
	_, err := provider.ListAllVersions(context.Background(), "kubernetes")
	if err != nil {
		t.Fatalf("First call error = %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 API call, got %d", callCount)
	}

	// Second call - should use cache
	_, err = provider.ListAllVersions(context.Background(), "kubernetes")
	if err != nil {
		t.Fatalf("Second call error = %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 API call (cached), got %d", callCount)
	}

	// Third call - should still use cache
	_, err = provider.GetVersionLifecycle(context.Background(), "kubernetes", "1.35")
	if err != nil {
		t.Fatalf("Third call error = %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 API call (cached), got %d", callCount)
	}
}

func TestProvider_CacheExpiration(t *testing.T) {
	callCount := 0
	mockClient := &MockClient{
		GetProductCyclesFunc: func(ctx context.Context, product string) ([]*ProductCycle, error) {
			callCount++
			return []*ProductCycle{
				{
					Cycle:       "1.35",
					ReleaseDate: "2025-11-19",
					Support:     "2027-12-19",
					EOL:         "2029-05-19",
				},
			}, nil
		},
	}

	// Very short TTL for testing
	provider := NewProvider(mockClient, 50*time.Millisecond)

	// First call
	_, err := provider.ListAllVersions(context.Background(), "kubernetes")
	if err != nil {
		t.Fatalf("First call error = %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 API call, got %d", callCount)
	}

	// Wait for cache to expire
	time.Sleep(100 * time.Millisecond)

	// Second call after expiration - should hit API again
	_, err = provider.ListAllVersions(context.Background(), "kubernetes")
	if err != nil {
		t.Fatalf("Second call error = %v", err)
	}
	if callCount != 2 {
		t.Errorf("Expected 2 API calls (cache expired), got %d", callCount)
	}
}

func TestProvider_UnsupportedEngine(t *testing.T) {
	mockClient := &MockClient{}
	provider := NewProvider(mockClient, 1*time.Hour)

	_, err := provider.GetVersionLifecycle(context.Background(), "unsupported-engine", "1.0")
	if err == nil {
		t.Error("Expected error for unsupported engine, got nil")
	}
}

func TestProvider_VersionNotFound(t *testing.T) {
	mockClient := &MockClient{
		GetProductCyclesFunc: func(ctx context.Context, product string) ([]*ProductCycle, error) {
			return []*ProductCycle{
				{
					Cycle:       "1.31",
					ReleaseDate: "2024-11-19",
					Support:     "2025-12-19",
					EOL:         "2027-05-19",
				},
			}, nil
		},
	}

	provider := NewProvider(mockClient, 1*time.Hour)

	lifecycle, err := provider.GetVersionLifecycle(context.Background(), "kubernetes", "99.99")
	if err != nil {
		t.Fatalf("Expected no error for unknown version, got %v", err)
	}

	// Should return unsupported lifecycle with empty Version (signals missing data, not unsupported version)
	if lifecycle.IsSupported {
		t.Error("Unknown version should not be supported")
	}
	if lifecycle.Version != "" {
		t.Errorf("Version = %s, want empty string (signals data gap)", lifecycle.Version)
	}
	if lifecycle.Engine != "kubernetes" {
		t.Errorf("Engine = %s, want kubernetes", lifecycle.Engine)
	}
}

func TestProvider_Name(t *testing.T) {
	provider := NewProvider(&MockClient{}, 1*time.Hour)
	if name := provider.Name(); name != "endoflife-date-api" {
		t.Errorf("Name() = %s, want endoflife-date-api", name)
	}
}

func TestProvider_Engines(t *testing.T) {
	provider := NewProvider(&MockClient{}, 1*time.Hour)
	engines := provider.Engines()

	// Check that common engines are present
	engineMap := make(map[string]bool)
	for _, e := range engines {
		engineMap[e] = true
	}

	requiredEngines := []string{"kubernetes", "eks", "postgres", "mysql", "redis"}
	for _, required := range requiredEngines {
		if !engineMap[required] {
			t.Errorf("Expected engine %s to be present", required)
		}
	}
}

func TestConvertCycle_ExtendedSupport(t *testing.T) {
	provider := NewProvider(&MockClient{}, 1*time.Hour)

	tests := []struct {
		name                    string
		cycle                   *ProductCycle
		wantIsExtendedSupport   bool
		wantExtendedSupportDate bool
	}{
		{
			name: "extended support as boolean true - in extended window",
			cycle: &ProductCycle{
				Cycle:           "1.32",
				ReleaseDate:     "2024-11-19",
				Support:         "2025-12-19", // Past (relative to 2026-04-08)
				EOL:             "2027-05-19", // Future
				ExtendedSupport: true,
			},
			wantIsExtendedSupport:   true, // In extended support window
			wantExtendedSupportDate: true, // Should have end date
		},
		{
			name: "extended support as date string - in extended window",
			cycle: &ProductCycle{
				Cycle:           "1.31",
				ReleaseDate:     "2024-05-29",
				Support:         "2025-06-29", // Past (relative to 2026-04-08)
				EOL:             "2027-11-29", // Future
				ExtendedSupport: "2027-11-29",
			},
			wantIsExtendedSupport:   true, // In extended support window
			wantExtendedSupportDate: true,
		},
		{
			name: "future version - in standard support",
			cycle: &ProductCycle{
				Cycle:           "1.35",
				ReleaseDate:     "2025-11-19",
				Support:         "2027-12-19", // Future
				EOL:             "2029-05-19", // Far future
				ExtendedSupport: true,
			},
			wantIsExtendedSupport:   false, // Not yet in extended support window
			wantExtendedSupportDate: true,  // Should have end date
		},
		{
			name: "no extended support - EOL",
			cycle: &ProductCycle{
				Cycle:           "1.25",
				ReleaseDate:     "2023-02-21",
				Support:         "2024-03-21", // Past
				EOL:             "2025-08-21", // Past (relative to 2026-04-08)
				ExtendedSupport: false,
			},
			wantIsExtendedSupport:   false,
			wantExtendedSupportDate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lifecycle, err := provider.convertCycle("kubernetes", "amazon-eks", tt.cycle)
			if err != nil {
				t.Fatalf("convertCycle() error = %v", err)
			}

			if lifecycle.IsExtendedSupport != tt.wantIsExtendedSupport {
				t.Errorf("IsExtendedSupport = %v, want %v", lifecycle.IsExtendedSupport, tt.wantIsExtendedSupport)
			}

			hasExtendedDate := lifecycle.ExtendedSupportEnd != nil
			if hasExtendedDate != tt.wantExtendedSupportDate {
				t.Errorf("Has ExtendedSupportEnd = %v, want %v", hasExtendedDate, tt.wantExtendedSupportDate)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		want    string
	}{
		{
			name:    "valid date",
			input:   "2024-11-19",
			wantErr: false,
			want:    "2024-11-19",
		},
		{
			name:    "boolean true",
			input:   "true",
			wantErr: true,
		},
		{
			name:    "boolean false",
			input:   "false",
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   "19-11-2024",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				formatted := parsed.Format("2006-01-02")
				if formatted != tt.want {
					t.Errorf("parseDate() = %s, want %s", formatted, tt.want)
				}
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		name    string
		engine  string
		version string
		want    string
	}{
		{
			name:    "kubernetes with k8s- prefix",
			engine:  "kubernetes",
			version: "k8s-1.31",
			want:    "1.31",
		},
		{
			name:    "kubernetes without prefix",
			engine:  "kubernetes",
			version: "1.31",
			want:    "1.31",
		},
		{
			name:    "eks with kubernetes- prefix",
			engine:  "eks",
			version: "kubernetes-1.31",
			want:    "1.31",
		},
		{
			name:    "postgres version",
			engine:  "postgres",
			version: "15.4",
			want:    "15.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeVersion(tt.engine, tt.version)
			if got != tt.want {
				t.Errorf("normalizeVersion() = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestProvider_InterfaceCompliance verifies that Provider implements eol.Provider interface
func TestProvider_InterfaceCompliance(t *testing.T) {
	var _ interface {
		GetVersionLifecycle(ctx context.Context, engine, version string) (*types.VersionLifecycle, error)
		ListAllVersions(ctx context.Context, engine string) ([]*types.VersionLifecycle, error)
		Name() string
		Engines() []string
	} = (*Provider)(nil)
}
