package service

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/fleetdm/fleet/v4/server/mock"
	"github.com/fleetdm/fleet/v4/server/ptr"
	"github.com/fleetdm/fleet/v4/server/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFleetDesktopSummary(t *testing.T) {
	t.Run("free implementation", func(t *testing.T) {
		ds := new(mock.Store)
		svc, ctx := newTestService(t, ds, nil, nil)
		sum, err := svc.GetFleetDesktopSummary(ctx)
		require.ErrorIs(t, err, fleet.ErrMissingLicense)
		require.Empty(t, sum)
	})

	t.Run("different app config values for managed host", func(t *testing.T) {
		ds := new(mock.Store)
		license := &fleet.LicenseInfo{Tier: fleet.TierPremium, Expiration: time.Now().Add(24 * time.Hour)}
		svc, ctx := newTestService(t, ds, nil, nil, &TestServerOpts{License: license, SkipCreateTestUsers: true})
		ds.FailingPoliciesCountFunc = func(ctx context.Context, host *fleet.Host) (uint, error) {
			return uint(1), nil
		}
		const expectedPlatform = "darwin"
		ds.HasSelfServiceSoftwareInstallersFunc = func(ctx context.Context, platform string, teamID *uint) (bool, error) {
			assert.Equal(t, expectedPlatform, platform)
			return true, nil
		}

		cases := []struct {
			mdm         fleet.MDM
			depAssigned bool
			out         fleet.DesktopNotifications
		}{
			{
				mdm: fleet.MDM{
					EnabledAndConfigured: true,
					MacOSMigration: fleet.MacOSMigration{
						Enable: true,
					},
				},
				depAssigned: true,
				out: fleet.DesktopNotifications{
					NeedsMDMMigration:      true,
					RenewEnrollmentProfile: false,
				},
			},
			{
				mdm: fleet.MDM{
					EnabledAndConfigured: true,
					MacOSMigration: fleet.MacOSMigration{
						Enable: true,
					},
				},
				depAssigned: false,
				out: fleet.DesktopNotifications{
					NeedsMDMMigration:      false,
					RenewEnrollmentProfile: false,
				},
			},
			{
				mdm: fleet.MDM{
					EnabledAndConfigured: false,
					MacOSMigration: fleet.MacOSMigration{
						Enable: true,
					},
				},
				depAssigned: true,
				out: fleet.DesktopNotifications{
					NeedsMDMMigration:      false,
					RenewEnrollmentProfile: false,
				},
			},
			{
				mdm: fleet.MDM{
					EnabledAndConfigured: true,
					MacOSMigration: fleet.MacOSMigration{
						Enable: false,
					},
				},
				depAssigned: true,
				out: fleet.DesktopNotifications{
					NeedsMDMMigration:      false,
					RenewEnrollmentProfile: false,
				},
			},
			{
				mdm: fleet.MDM{
					EnabledAndConfigured: false,
					MacOSMigration: fleet.MacOSMigration{
						Enable: false,
					},
				},
				depAssigned: true,
				out: fleet.DesktopNotifications{
					NeedsMDMMigration:      false,
					RenewEnrollmentProfile: false,
				},
			},
		}

		for _, c := range cases {
			c := c
			ds.AppConfigFunc = func(ctx context.Context) (*fleet.AppConfig, error) {
				appCfg := fleet.AppConfig{}
				appCfg.MDM = c.mdm
				return &appCfg, nil
			}

			ds.IsHostConnectedToFleetMDMFunc = func(ctx context.Context, host *fleet.Host) (bool, error) {
				return false, nil
			}
			ds.GetHostMDMFunc = func(ctx context.Context, hostID uint) (*fleet.HostMDM, error) {
				return &fleet.HostMDM{
					IsServer:               false,
					InstalledFromDep:       true,
					Enrolled:               true,
					Name:                   fleet.WellKnownMDMIntune,
					DEPProfileAssignStatus: ptr.String(string(fleet.DEPAssignProfileResponseSuccess)),
				}, nil
			}

			ctx := test.HostContext(ctx, &fleet.Host{
				OsqueryHostID:      ptr.String("test"),
				DEPAssignedToFleet: &c.depAssigned,
				Platform:           expectedPlatform,
			})
			sum, err := svc.GetFleetDesktopSummary(ctx)
			require.NoError(t, err)
			require.Equal(t, c.out, sum.Notifications, fmt.Sprintf("enabled_and_configured: %t | macos_migration.enable: %t", c.mdm.EnabledAndConfigured, c.mdm.MacOSMigration.Enable))
			require.EqualValues(t, 1, *sum.FailingPolicies)
			assert.Equal(t, ptr.Bool(true), sum.SelfService)
		}

	})

	t.Run("different app config values for unmanaged host", func(t *testing.T) {
		ds := new(mock.Store)
		license := &fleet.LicenseInfo{Tier: fleet.TierPremium, Expiration: time.Now().Add(24 * time.Hour)}
		svc, ctx := newTestService(t, ds, nil, nil, &TestServerOpts{License: license, SkipCreateTestUsers: true})
		ds.FailingPoliciesCountFunc = func(ctx context.Context, host *fleet.Host) (uint, error) {
			return uint(1), nil
		}
		ds.HasSelfServiceSoftwareInstallersFunc = func(ctx context.Context, platform string, teamID *uint) (bool, error) {
			return true, nil
		}
		cases := []struct {
			mdm         fleet.MDM
			depAssigned bool
			out         fleet.DesktopNotifications
		}{
			{
				mdm: fleet.MDM{
					EnabledAndConfigured: true,
					MacOSMigration: fleet.MacOSMigration{
						Enable: true,
					},
				},
				depAssigned: true,
				out: fleet.DesktopNotifications{
					NeedsMDMMigration:      false,
					RenewEnrollmentProfile: true,
				},
			},
			{
				mdm: fleet.MDM{
					EnabledAndConfigured: false,
					MacOSMigration: fleet.MacOSMigration{
						Enable: true,
					},
				},
				depAssigned: true,
				out: fleet.DesktopNotifications{
					NeedsMDMMigration:      false,
					RenewEnrollmentProfile: false,
				},
			},
			{
				mdm: fleet.MDM{
					EnabledAndConfigured: true,
					MacOSMigration: fleet.MacOSMigration{
						Enable: false,
					},
				},
				depAssigned: true,
				out: fleet.DesktopNotifications{
					NeedsMDMMigration:      false,
					RenewEnrollmentProfile: false,
				},
			},
			{
				mdm: fleet.MDM{
					EnabledAndConfigured: false,
					MacOSMigration: fleet.MacOSMigration{
						Enable: false,
					},
				},
				depAssigned: true,
				out: fleet.DesktopNotifications{
					NeedsMDMMigration:      false,
					RenewEnrollmentProfile: false,
				},
			},
		}

		mdmInfo := &fleet.HostMDM{
			IsServer:               false,
			InstalledFromDep:       true,
			Enrolled:               false,
			Name:                   fleet.WellKnownMDMFleet,
			DEPProfileAssignStatus: ptr.String(string(fleet.DEPAssignProfileResponseSuccess)),
		}

		for _, c := range cases {
			c := c
			ds.AppConfigFunc = func(ctx context.Context) (*fleet.AppConfig, error) {
				appCfg := fleet.AppConfig{}
				appCfg.MDM = c.mdm
				return &appCfg, nil
			}
			ds.IsHostConnectedToFleetMDMFunc = func(ctx context.Context, host *fleet.Host) (bool, error) {
				return false, nil
			}

			ds.GetHostMDMFunc = func(ctx context.Context, hostID uint) (*fleet.HostMDM, error) {
				return mdmInfo, nil
			}

			ctx = test.HostContext(ctx, &fleet.Host{
				OsqueryHostID:      ptr.String("test"),
				DEPAssignedToFleet: &c.depAssigned,
			})
			sum, err := svc.GetFleetDesktopSummary(ctx)
			require.NoError(t, err)
			require.Equal(t, c.out, sum.Notifications, fmt.Sprintf("enabled_and_configured: %t | macos_migration.enable: %t", c.mdm.EnabledAndConfigured, c.mdm.MacOSMigration.Enable))
			require.EqualValues(t, 1, *sum.FailingPolicies)
		}

	})

	t.Run("different host attributes", func(t *testing.T) {
		ds := new(mock.Store)
		license := &fleet.LicenseInfo{Tier: fleet.TierPremium, Expiration: time.Now().Add(24 * time.Hour)}
		svc, ctx := newTestService(t, ds, nil, nil, &TestServerOpts{License: license, SkipCreateTestUsers: true})

		// context without a host
		sum, err := svc.GetFleetDesktopSummary(ctx)
		require.Empty(t, sum)
		var authErr *fleet.AuthRequiredError
		require.ErrorAs(t, err, &authErr)

		ds.FailingPoliciesCountFunc = func(ctx context.Context, host *fleet.Host) (uint, error) {
			return uint(1), nil
		}

		ds.AppConfigFunc = func(ctx context.Context) (*fleet.AppConfig, error) {
			appCfg := fleet.AppConfig{}
			appCfg.MDM.EnabledAndConfigured = true
			appCfg.MDM.MacOSMigration.Enable = true
			return &appCfg, nil
		}

		ds.HasSelfServiceSoftwareInstallersFunc = func(ctx context.Context, platform string, teamID *uint) (bool, error) {
			return false, nil
		}

		cases := []struct {
			name    string
			host    *fleet.Host
			hostMDM *fleet.HostMDM
			err     error
			out     fleet.DesktopNotifications
		}{
			{
				name: "not enrolled into osquery",
				host: &fleet.Host{OsqueryHostID: nil},
				err:  nil,
				out: fleet.DesktopNotifications{
					NeedsMDMMigration:      false,
					RenewEnrollmentProfile: false,
				},
			},
			{
				name: "manually enrolled into another MDM",
				host: &fleet.Host{
					OsqueryHostID:      ptr.String("test"),
					DEPAssignedToFleet: ptr.Bool(false),
				},
				hostMDM: &fleet.HostMDM{
					IsServer:               false,
					InstalledFromDep:       false,
					Enrolled:               true,
					Name:                   fleet.WellKnownMDMIntune,
					DEPProfileAssignStatus: ptr.String(string(fleet.DEPAssignProfileResponseSuccess)),
				},
				err: nil,
				out: fleet.DesktopNotifications{
					NeedsMDMMigration:      false,
					RenewEnrollmentProfile: false,
				},
			},
			{
				name: "DEP capable, but already unenrolled",
				host: &fleet.Host{
					DEPAssignedToFleet: ptr.Bool(true),
					OsqueryHostID:      ptr.String("test"),
				},
				hostMDM: &fleet.HostMDM{
					IsServer:               false,
					InstalledFromDep:       true,
					Enrolled:               false,
					Name:                   fleet.WellKnownMDMFleet,
					DEPProfileAssignStatus: ptr.String(string(fleet.DEPAssignProfileResponseSuccess)),
				},
				err: nil,
				out: fleet.DesktopNotifications{
					NeedsMDMMigration:      false,
					RenewEnrollmentProfile: true,
				},
			},
			{
				name: "DEP capable, but enrolled into Fleet",
				host: &fleet.Host{
					DEPAssignedToFleet: ptr.Bool(true),
					OsqueryHostID:      ptr.String("test"),
				},
				hostMDM: &fleet.HostMDM{
					IsServer:               false,
					InstalledFromDep:       true,
					Enrolled:               true,
					Name:                   fleet.WellKnownMDMFleet,
					DEPProfileAssignStatus: ptr.String(string(fleet.DEPAssignProfileResponseSuccess)),
				},
				err: nil,
				out: fleet.DesktopNotifications{
					NeedsMDMMigration:      false,
					RenewEnrollmentProfile: false,
				},
			},
			{
				name: "failed ADE assignment status",
				host: &fleet.Host{
					DEPAssignedToFleet: ptr.Bool(true),
					OsqueryHostID:      ptr.String("test"),
				},
				hostMDM: &fleet.HostMDM{
					IsServer:               false,
					InstalledFromDep:       true,
					Enrolled:               true,
					Name:                   fleet.WellKnownMDMIntune,
					DEPProfileAssignStatus: ptr.String(string(fleet.DEPAssignProfileResponseFailed)),
				},
				err: nil,
				out: fleet.DesktopNotifications{
					NeedsMDMMigration:      false,
					RenewEnrollmentProfile: false,
				},
			},
			{
				name: "not accessible ADE assignment status",
				host: &fleet.Host{
					DEPAssignedToFleet: ptr.Bool(true),
					OsqueryHostID:      ptr.String("test"),
				},
				hostMDM: &fleet.HostMDM{
					IsServer:               false,
					InstalledFromDep:       true,
					Enrolled:               true,
					Name:                   fleet.WellKnownMDMIntune,
					DEPProfileAssignStatus: ptr.String(string(fleet.DEPAssignProfileResponseNotAccessible)),
				},
				err: nil,
				out: fleet.DesktopNotifications{
					NeedsMDMMigration:      false,
					RenewEnrollmentProfile: false,
				},
			},
			{
				name: "empty ADE assignment status",
				host: &fleet.Host{
					DEPAssignedToFleet: ptr.Bool(true),
					OsqueryHostID:      ptr.String("test"),
				},
				hostMDM: &fleet.HostMDM{
					IsServer:               false,
					InstalledFromDep:       true,
					Enrolled:               true,
					Name:                   fleet.WellKnownMDMIntune,
					DEPProfileAssignStatus: ptr.String(""),
				},
				err: nil,
				out: fleet.DesktopNotifications{
					NeedsMDMMigration:      false,
					RenewEnrollmentProfile: false,
				},
			},
			{
				name: "nil ADE assignment status",
				host: &fleet.Host{
					DEPAssignedToFleet: ptr.Bool(true),
					OsqueryHostID:      ptr.String("test"),
				},
				hostMDM: &fleet.HostMDM{
					IsServer:               false,
					InstalledFromDep:       true,
					Enrolled:               true,
					Name:                   fleet.WellKnownMDMIntune,
					DEPProfileAssignStatus: nil,
				},
				err: nil,
				out: fleet.DesktopNotifications{
					NeedsMDMMigration:      false,
					RenewEnrollmentProfile: false,
				},
			},
			{
				name: "all conditions met",
				host: &fleet.Host{
					DEPAssignedToFleet: ptr.Bool(true),
					OsqueryHostID:      ptr.String("test"),
				},
				hostMDM: &fleet.HostMDM{
					IsServer:               false,
					InstalledFromDep:       true,
					Enrolled:               true,
					Name:                   fleet.WellKnownMDMIntune,
					DEPProfileAssignStatus: ptr.String(string(fleet.DEPAssignProfileResponseSuccess)),
				},
				err: nil,
				out: fleet.DesktopNotifications{
					NeedsMDMMigration:      true,
					RenewEnrollmentProfile: false,
				},
			},
		}

		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				ctx = test.HostContext(ctx, c.host)

				ds.GetHostMDMFunc = func(ctx context.Context, hostID uint) (*fleet.HostMDM, error) {
					if c.hostMDM == nil {
						return nil, sql.ErrNoRows
					}
					return c.hostMDM, nil
				}

				ds.IsHostConnectedToFleetMDMFunc = func(ctx context.Context, host *fleet.Host) (bool, error) {
					return c.hostMDM != nil && c.hostMDM.Enrolled == true && c.hostMDM.Name == fleet.WellKnownMDMFleet, nil
				}

				sum, err := svc.GetFleetDesktopSummary(ctx)

				if c.err != nil {
					require.ErrorIs(t, err, c.err)
					require.Empty(t, sum)
				} else {

					require.NoError(t, err)
					require.Equal(t, c.out, sum.Notifications)
					require.EqualValues(t, 1, *sum.FailingPolicies)
				}
			})
		}

	})
}
