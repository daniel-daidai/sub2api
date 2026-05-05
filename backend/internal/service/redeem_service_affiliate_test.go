//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type redeemAffiliateRepoStub struct {
	inviteeSummary *AffiliateSummary
	inviterSummary *AffiliateSummary
	accrueCalls    int
	accrueInviter  int64
	accrueInvitee  int64
	accrueAmount   float64
}

func (s *redeemAffiliateRepoStub) EnsureUserAffiliate(_ context.Context, userID int64) (*AffiliateSummary, error) {
	if s.inviteeSummary != nil && s.inviteeSummary.UserID == userID {
		return s.inviteeSummary, nil
	}
	if s.inviterSummary != nil && s.inviterSummary.UserID == userID {
		return s.inviterSummary, nil
	}
	return &AffiliateSummary{UserID: userID}, nil
}

func (s *redeemAffiliateRepoStub) GetAffiliateByCode(context.Context, string) (*AffiliateSummary, error) {
	panic("unexpected GetAffiliateByCode call")
}

func (s *redeemAffiliateRepoStub) BindInviter(context.Context, int64, int64) (bool, error) {
	panic("unexpected BindInviter call")
}

func (s *redeemAffiliateRepoStub) AccrueQuota(_ context.Context, inviterID, inviteeUserID int64, amount float64, _ int, _ *int64) (bool, error) {
	s.accrueCalls++
	s.accrueInviter = inviterID
	s.accrueInvitee = inviteeUserID
	s.accrueAmount = amount
	return true, nil
}

func (s *redeemAffiliateRepoStub) GetAccruedRebateFromInvitee(context.Context, int64, int64) (float64, error) {
	return 0, nil
}

func (s *redeemAffiliateRepoStub) ThawFrozenQuota(context.Context, int64) (float64, error) {
	return 0, nil
}

func (s *redeemAffiliateRepoStub) TransferQuotaToBalance(context.Context, int64, float64) (float64, float64, error) {
	panic("unexpected TransferQuotaToBalance call")
}

func (s *redeemAffiliateRepoStub) ListInvitees(context.Context, int64, int) ([]AffiliateInvitee, error) {
	panic("unexpected ListInvitees call")
}

func (s *redeemAffiliateRepoStub) UpdateUserAffCode(context.Context, int64, string) error {
	panic("unexpected UpdateUserAffCode call")
}

func (s *redeemAffiliateRepoStub) ResetUserAffCode(context.Context, int64) (string, error) {
	panic("unexpected ResetUserAffCode call")
}

func (s *redeemAffiliateRepoStub) SetUserRebateRate(context.Context, int64, *float64) error {
	panic("unexpected SetUserRebateRate call")
}

func (s *redeemAffiliateRepoStub) BatchSetUserRebateRate(context.Context, []int64, *float64) error {
	panic("unexpected BatchSetUserRebateRate call")
}

func (s *redeemAffiliateRepoStub) ListUsersWithCustomSettings(context.Context, AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error) {
	panic("unexpected ListUsersWithCustomSettings call")
}

func newRedeemAffiliateServiceStub() *AffiliateService {
	repo := &redeemAffiliateRepoStub{
		inviteeSummary: &AffiliateSummary{
			UserID:    1,
			InviterID: ptrInt64Affiliate(2),
		},
		inviterSummary: &AffiliateSummary{
			UserID: 2,
		},
	}
	return &AffiliateService{
		repo: repo,
		settingService: affiliateSettingServiceStub(map[string]string{
			SettingKeyAffiliateEnabled:             "true",
			SettingKeyAffiliateRebateRate:          "10",
			SettingKeyAffiliateRebateFreezeHours:   "0",
			SettingKeyAffiliateRebateDurationDays:  "0",
			SettingKeyAffiliateRebatePerInviteeCap: "0",
			SettingKeyAffiliateTransferThreshold:   "5",
		}),
	}
}

func ptrInt64Affiliate(v int64) *int64 {
	return &v
}

func TestRedeemBalanceCode_AccruesAffiliateRebate(t *testing.T) {
	client := newPaymentOrderLifecycleTestClient(t)
	userRepo := &mockUserRepo{
		getByIDUser: &User{
			ID:      1,
			Balance: 0,
		},
	}
	userRepo.updateBalanceFn = func(ctx context.Context, id int64, amount float64) error {
		require.Equal(t, int64(1), id)
		userRepo.getByIDUser.Balance += amount
		return nil
	}

	redeemRepo := &paymentOrderLifecycleRedeemRepo{
		codesByCode: map[string]*RedeemCode{
			"BAL-AFF": {
				ID:     1,
				Code:   "BAL-AFF",
				Type:   RedeemTypeBalance,
				Value:  12.5,
				Status: StatusUnused,
			},
		},
	}

	affiliateSvc := newRedeemAffiliateServiceStub()
	affiliateRepo := affiliateSvc.repo.(*redeemAffiliateRepoStub)

	redeemService := NewRedeemService(
		redeemRepo,
		userRepo,
		nil,
		nil,
		nil,
		client,
		nil,
	)
	redeemService.SetAffiliateService(affiliateSvc)

	_, err := redeemService.Redeem(context.Background(), 1, "BAL-AFF")
	require.NoError(t, err)
	require.Equal(t, 1, affiliateRepo.accrueCalls)
	require.Equal(t, int64(2), affiliateRepo.accrueInviter)
	require.Equal(t, int64(1), affiliateRepo.accrueInvitee)
	require.InDelta(t, 1.25, affiliateRepo.accrueAmount, 1e-9)
}

func TestRedeemBalanceCode_SkipAffiliateRebateWhenContextMarked(t *testing.T) {
	client := newPaymentOrderLifecycleTestClient(t)
	userRepo := &mockUserRepo{
		getByIDUser: &User{
			ID:      1,
			Balance: 0,
		},
	}
	userRepo.updateBalanceFn = func(ctx context.Context, id int64, amount float64) error {
		require.Equal(t, int64(1), id)
		userRepo.getByIDUser.Balance += amount
		return nil
	}

	redeemRepo := &paymentOrderLifecycleRedeemRepo{
		codesByCode: map[string]*RedeemCode{
			"BAL-SKIP": {
				ID:     2,
				Code:   "BAL-SKIP",
				Type:   RedeemTypeBalance,
				Value:  12.5,
				Status: StatusUnused,
			},
		},
	}

	affiliateSvc := newRedeemAffiliateServiceStub()
	affiliateRepo := affiliateSvc.repo.(*redeemAffiliateRepoStub)

	redeemService := NewRedeemService(
		redeemRepo,
		userRepo,
		nil,
		nil,
		nil,
		client,
		nil,
	)
	redeemService.SetAffiliateService(affiliateSvc)

	_, err := redeemService.Redeem(withSkipAffiliateRebate(context.Background()), 1, "BAL-SKIP")
	require.NoError(t, err)
	require.Equal(t, 0, affiliateRepo.accrueCalls)
}
