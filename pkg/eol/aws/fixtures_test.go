package aws

import (
	"time"
)

// AWSAPIFixtures contains realistic AWS RDS API response data for testing
//
//nolint:govet // field alignment sacrificed for test data organization
var AWSAPIFixtures = struct {
	AuroraMySQLVersions      []*EngineVersion
	AuroraPostgreSQLVersions []*EngineVersion
	DeprecatedMySQLVersion   *EngineVersion
	CurrentMySQLVersion      *EngineVersion
	ExtendedSupportVersion   *EngineVersion
	VersionWithNoUpgradePath *EngineVersion
}{
	// Aurora MySQL versions (realistic AWS API responses)
	AuroraMySQLVersions: []*EngineVersion{
		{
			Engine:               "aurora-mysql",
			EngineVersion:        "5.6.10a",
			Status:               "deprecated",
			SupportedEngineModes: []string{"provisioned"},
			ValidUpgradeTarget: []string{
				"5.7.12",
				"5.7.mysql_aurora.2.04.2",
			},
			DeprecationDate:         timePtr(time.Date(2021, 2, 28, 0, 0, 0, 0, time.UTC)),
			SupportsClusters:        true,
			SupportsGlobalDatabases: false,
		},
		{
			Engine:               "aurora-mysql",
			EngineVersion:        "5.7.12",
			Status:               "available",
			SupportedEngineModes: []string{"provisioned"},
			ValidUpgradeTarget: []string{
				"5.7.mysql_aurora.2.04.2",
				"8.0.mysql_aurora.3.01.0",
			},
			DeprecationDate:         timePtr(time.Date(2024, 10, 31, 0, 0, 0, 0, time.UTC)),
			SupportsClusters:        true,
			SupportsGlobalDatabases: true,
		},
		{
			Engine:               "aurora-mysql",
			EngineVersion:        "5.7.mysql_aurora.2.04.2",
			Status:               "available",
			SupportedEngineModes: []string{"provisioned"},
			ValidUpgradeTarget: []string{
				"5.7.mysql_aurora.2.11.3",
				"8.0.mysql_aurora.3.01.0",
			},
			DeprecationDate:         timePtr(time.Date(2024, 10, 31, 0, 0, 0, 0, time.UTC)),
			SupportsClusters:        true,
			SupportsGlobalDatabases: true,
		},
		{
			Engine:               "aurora-mysql",
			EngineVersion:        "8.0.mysql_aurora.3.01.0",
			Status:               "available",
			SupportedEngineModes: []string{"provisioned"},
			ValidUpgradeTarget: []string{
				"8.0.mysql_aurora.3.05.2",
			},
			SupportsClusters:        true,
			SupportsGlobalDatabases: true,
		},
		{
			Engine:                  "aurora-mysql",
			EngineVersion:           "8.0.mysql_aurora.3.05.2",
			Status:                  "available",
			SupportedEngineModes:    []string{"provisioned"},
			ValidUpgradeTarget:      []string{},
			SupportsClusters:        true,
			SupportsGlobalDatabases: true,
		},
	},

	// Aurora PostgreSQL versions
	AuroraPostgreSQLVersions: []*EngineVersion{
		{
			Engine:               "aurora-postgresql",
			EngineVersion:        "11.21",
			Status:               "deprecated",
			SupportedEngineModes: []string{"provisioned"},
			ValidUpgradeTarget: []string{
				"12.16",
				"13.12",
			},
			DeprecationDate:         timePtr(time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC)),
			SupportsClusters:        true,
			SupportsGlobalDatabases: true,
		},
		{
			Engine:               "aurora-postgresql",
			EngineVersion:        "13.12",
			Status:               "available",
			SupportedEngineModes: []string{"provisioned"},
			ValidUpgradeTarget: []string{
				"14.9",
				"15.4",
			},
			SupportsClusters:        true,
			SupportsGlobalDatabases: true,
		},
		{
			Engine:                  "aurora-postgresql",
			EngineVersion:           "15.4",
			Status:                  "available",
			SupportedEngineModes:    []string{"provisioned", "serverless"},
			ValidUpgradeTarget:      []string{},
			SupportsClusters:        true,
			SupportsGlobalDatabases: true,
		},
	},

	// Individual test fixtures
	DeprecatedMySQLVersion: &EngineVersion{
		Engine:               "aurora-mysql",
		EngineVersion:        "5.6.10a",
		Status:               "deprecated",
		SupportedEngineModes: []string{"provisioned"},
		ValidUpgradeTarget:   []string{"5.7.12"},
		DeprecationDate:      timePtr(time.Date(2021, 2, 28, 0, 0, 0, 0, time.UTC)),
		SupportsClusters:     true,
	},

	CurrentMySQLVersion: &EngineVersion{
		Engine:                  "aurora-mysql",
		EngineVersion:           "8.0.mysql_aurora.3.05.2",
		Status:                  "available",
		SupportedEngineModes:    []string{"provisioned"},
		ValidUpgradeTarget:      []string{},
		SupportsClusters:        true,
		SupportsGlobalDatabases: true,
	},

	ExtendedSupportVersion: &EngineVersion{
		Engine:               "aurora-mysql",
		EngineVersion:        "5.7.12",
		Status:               "available",
		SupportedEngineModes: []string{"provisioned"},
		ValidUpgradeTarget:   []string{"8.0.mysql_aurora.3.01.0"},
		// Deprecation date in the past, but still "available" = extended support
		DeprecationDate:  timePtr(time.Date(2024, 10, 31, 0, 0, 0, 0, time.UTC)),
		SupportsClusters: true,
	},

	VersionWithNoUpgradePath: &EngineVersion{
		Engine:               "aurora-mysql",
		EngineVersion:        "5.6.10a",
		Status:               "deprecated",
		SupportedEngineModes: []string{"provisioned"},
		ValidUpgradeTarget:   []string{}, // No upgrade path = EOL
		DeprecationDate:      timePtr(time.Date(2021, 2, 28, 0, 0, 0, 0, time.UTC)),
		SupportsClusters:     true,
	},
}

// Helper function to create time pointers
func timePtr(t time.Time) *time.Time {
	return &t
}
