//go:build unit

package service

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

type affiliateSettingRepoStub struct {
	value  string
	values map[string]string
	err    error
}

func (s *affiliateSettingRepoStub) Get(context.Context, string) (*Setting, error) { return nil, s.err }
func (s *affiliateSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if s.values != nil {
		return s.values[key], nil
	}
	return s.value, nil
}
func (s *affiliateSettingRepoStub) Set(context.Context, string, string) error { return s.err }
func (s *affiliateSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	return map[string]string{}, nil
}
func (s *affiliateSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	return s.err
}
func (s *affiliateSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	return map[string]string{}, nil
}
func (s *affiliateSettingRepoStub) Delete(context.Context, string) error { return s.err }

func affiliateSettingServiceStub(values map[string]string) *SettingService {
	return &SettingService{settingRepo: &affiliateSettingRepoStub{values: values}}
}

func TestResolveRebateRatePercent_PerUserOverride(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{}

	require.InDelta(t, AffiliateRebateRateDefault,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{}), 1e-9)

	rate := 50.0
	require.InDelta(t, 50.0,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &rate}), 1e-9)

	zero := 0.0
	require.InDelta(t, 0.0,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &zero}), 1e-9)

	tooHigh := 250.0
	require.InDelta(t, AffiliateRebateRateMax,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &tooHigh}), 1e-9)

	tooLow := -5.0
	require.InDelta(t, AffiliateRebateRateMin,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &tooLow}), 1e-9)
}

func TestAffiliateRebateRatePercentSemantics(t *testing.T) {
	t.Parallel()

	svc := &AffiliateService{settingService: affiliateSettingServiceStub(map[string]string{
		SettingKeyAffiliateRebateRate: "1",
	})}
	require.Equal(t, 1.0, svc.loadAffiliateRebateRatePercent(context.Background()))

	svc.settingService = affiliateSettingServiceStub(map[string]string{
		SettingKeyAffiliateRebateRate: "0.2",
	})
	require.Equal(t, 0.2, svc.loadAffiliateRebateRatePercent(context.Background()))

	svc.settingService = affiliateSettingServiceStub(map[string]string{})
	require.Equal(t, AffiliateRebateRateDefault, svc.loadAffiliateRebateRatePercent(context.Background()))
}

func TestIsEnabled_NilSettingServiceReturnsDefault(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{}
	require.False(t, svc.IsEnabled(context.Background()))
	require.Equal(t, AffiliateEnabledDefault, svc.IsEnabled(context.Background()))
}

func TestValidateExclusiveRate_BoundaryAndInvalid(t *testing.T) {
	t.Parallel()
	require.NoError(t, validateExclusiveRate(nil))

	for _, v := range []float64{0, 0.01, 50, 99.99, 100} {
		v := v
		require.NoError(t, validateExclusiveRate(&v), "value %v should be valid", v)
	}

	for _, v := range []float64{-0.01, 100.01, -100, 200} {
		v := v
		require.Error(t, validateExclusiveRate(&v), "value %v should be rejected", v)
	}

	nan := math.NaN()
	require.Error(t, validateExclusiveRate(&nan))
	posInf := math.Inf(1)
	require.Error(t, validateExclusiveRate(&posInf))
	negInf := math.Inf(-1)
	require.Error(t, validateExclusiveRate(&negInf))
}

type affiliateRepoStub struct {
	summary           *AffiliateSummary
	codeSummary       *AffiliateSummary
	bindCalled        bool
	bindUserID        int64
	bindInviterID     int64
	bindResult        bool
	bindErr           error
	transferCalled    bool
	transferUserID    int64
	transferMinAmount float64
	transferAmount    float64
	transferBalance   float64
	transferErr       error
	invitees          []AffiliateInvitee
}

func (s *affiliateRepoStub) EnsureUserAffiliate(context.Context, int64) (*AffiliateSummary, error) {
	return s.summary, nil
}

func (s *affiliateRepoStub) GetAffiliateByCode(context.Context, string) (*AffiliateSummary, error) {
	return s.codeSummary, nil
}

func (s *affiliateRepoStub) BindInviter(_ context.Context, userID, inviterID int64) (bool, error) {
	s.bindCalled = true
	s.bindUserID = userID
	s.bindInviterID = inviterID
	return s.bindResult, s.bindErr
}

func (s *affiliateRepoStub) AccrueQuota(context.Context, int64, int64, float64, int, *int64) (bool, error) {
	return true, nil
}

func (s *affiliateRepoStub) GetAccruedRebateFromInvitee(context.Context, int64, int64) (float64, error) {
	return 0, nil
}

func (s *affiliateRepoStub) ThawFrozenQuota(context.Context, int64) (float64, error) {
	return 0, nil
}

func (s *affiliateRepoStub) TransferQuotaToBalance(_ context.Context, userID int64, minAmount float64) (float64, float64, error) {
	s.transferCalled = true
	s.transferUserID = userID
	s.transferMinAmount = minAmount
	return s.transferAmount, s.transferBalance, s.transferErr
}

func (s *affiliateRepoStub) ListInvitees(context.Context, int64, int) ([]AffiliateInvitee, error) {
	return s.invitees, nil
}

func (s *affiliateRepoStub) UpdateUserAffCode(context.Context, int64, string) error { return nil }
func (s *affiliateRepoStub) ResetUserAffCode(context.Context, int64) (string, error) {
	return "", nil
}
func (s *affiliateRepoStub) SetUserRebateRate(context.Context, int64, *float64) error {
	return nil
}
func (s *affiliateRepoStub) BatchSetUserRebateRate(context.Context, []int64, *float64) error {
	return nil
}
func (s *affiliateRepoStub) ListUsersWithCustomSettings(context.Context, AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error) {
	return nil, 0, nil
}

func TestBindInviterByCode_UsesSignupRewardSetting(t *testing.T) {
	t.Parallel()

	repo := &affiliateRepoStub{
		summary: &AffiliateSummary{
			UserID:  2,
			AffCode: "SELFCODE0001",
		},
		codeSummary: &AffiliateSummary{
			UserID:  1,
			AffCode: "ABCDEFGHJKLM",
		},
		bindResult: true,
	}
	svc := &AffiliateService{
		repo: repo,
		settingService: affiliateSettingServiceStub(map[string]string{
			SettingKeyAffiliateEnabled: "true",
		}),
	}

	err := svc.BindInviterByCode(context.Background(), 2, "ABCDEFGHJKLM")
	require.NoError(t, err)
	require.True(t, repo.bindCalled)
	require.Equal(t, int64(2), repo.bindUserID)
	require.Equal(t, int64(1), repo.bindInviterID)
}

func TestTransferAffiliateQuota_RequiresThreshold(t *testing.T) {
	t.Parallel()

	repo := &affiliateRepoStub{
		summary: &AffiliateSummary{
			UserID:   1,
			AffCode:  "ABCDEFGHJKLM",
			AffQuota: 4.5,
		},
	}
	svc := &AffiliateService{
		repo: repo,
		settingService: affiliateSettingServiceStub(map[string]string{
			SettingKeyAffiliateTransferThreshold: "5",
		}),
	}

	_, _, err := svc.TransferAffiliateQuota(context.Background(), 1)
	require.ErrorIs(t, err, ErrAffiliateQuotaThreshold)
	require.False(t, repo.transferCalled)
}

func TestGetAffiliateDetail_ExposesProgramSettings(t *testing.T) {
	t.Parallel()

	repo := &affiliateRepoStub{
		summary: &AffiliateSummary{
			UserID:          1,
			AffCode:         "ABCDEFGHJKLM",
			AffQuota:        3,
			AffHistoryQuota: 7,
		},
	}
	svc := &AffiliateService{
		repo: repo,
		settingService: affiliateSettingServiceStub(map[string]string{
			SettingKeyAffiliateRebateRate:        "10",
			SettingKeyAffiliateTransferThreshold: "5",
		}),
	}

	detail, err := svc.GetAffiliateDetail(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 10.0, detail.RebateRate)
	require.Equal(t, 10.0, detail.EffectiveRebateRatePercent)
	require.Equal(t, AffiliateTransferThresholdDefault, detail.TransferMin)
	require.Equal(t, 2.0, detail.TransferGap)
	require.False(t, detail.CanTransfer)
}

func TestMaskEmail(t *testing.T) {
	t.Parallel()
	require.Equal(t, "a***@g***.com", maskEmail("alice@gmail.com"))
	require.Equal(t, "x***@d***", maskEmail("x@domain"))
	require.Equal(t, "", maskEmail(""))
}

func TestIsValidAffiliateCodeFormat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"valid canonical 12-char", "ABCDEFGHJKLM", true},
		{"valid all digits 2-9", "234567892345", true},
		{"valid mixed", "A2B3C4D5E6F7", true},
		{"valid admin custom short", "VIP1", true},
		{"valid admin custom with hyphen", "NEW-USER", true},
		{"valid admin custom with underscore", "VIP_2026", true},
		{"valid 32-char max", "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345", true},
		{"letter I now allowed", "IBCDEFGHJKLM", true},
		{"letter O now allowed", "OBCDEFGHJKLM", true},
		{"digit 0 now allowed", "0BCDEFGHJKLM", true},
		{"digit 1 now allowed", "1BCDEFGHJKLM", true},
		{"too short (3 chars)", "ABC", false},
		{"too long (33 chars)", "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456", false},
		{"lowercase rejected (caller must ToUpper first)", "abcdefghjklm", false},
		{"empty", "", false},
		{"utf8 non-ascii", "\u00c4\u00c4\u00c4\u00c4\u00c4\u00c4", false},
		{"ascii punctuation .", "ABCDEFGHJK.M", false},
		{"whitespace", "ABCDEFGHJK M", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isValidAffiliateCodeFormat(tc.in))
		})
	}
}
