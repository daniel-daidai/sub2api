package service

import "math"

const (
	AffiliateTransferThresholdDefault = 5.0
	AffiliateTransferThresholdMin     = 0.0
	AffiliateTransferThresholdMax     = math.MaxFloat64

	SettingKeyAffiliateTransferThreshold = "affiliate_transfer_threshold"
)
