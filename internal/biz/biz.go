package biz

import "github.com/google/wire"

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(
	NewBillingConfig,
	NewUserBalanceUseCase,
	NewFreeQuotaUseCase,
	NewBillingRecordUseCase,
	NewRechargeOrderUseCase,
	NewStatsUseCase,
	NewBillingUseCase, // 组合 UseCase
)

