package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// Returns exact before/after values for administrative adjustment auditing.
func AdjustWalletWithReceipt(userId int, mode string, value int) (before, after int, err error) {
	if value < -common.MaxWalletQuota {
		return 0, 0, fmt.Errorf("wallet amount out of range")
	}
	if err = common.ValidateWalletQuota(value); err != nil {
		return
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).First(&user, userId).Error; err != nil {
			return err
		}
		before = user.Quota
		if before < -common.MaxWalletQuota || before > common.MaxWalletQuota {
			return fmt.Errorf("wallet balance out of range")
		}
		switch mode {
		case "add":
			if value <= 0 || before > common.MaxWalletQuota-value {
				return fmt.Errorf("invalid wallet addition")
			}
			after = before + value
		case "subtract":
			if value <= 0 || before < -common.MaxWalletQuota+value {
				return fmt.Errorf("invalid wallet subtraction")
			}
			after = before - value
		case "override":
			after = value
		default:
			return fmt.Errorf("invalid adjustment mode")
		}
		return tx.Model(&user).Update("quota", after).Error
	})
	if err == nil && common.RedisEnabled {
		if cacheErr := cacheIncrUserQuota(userId, int64(after-before)); cacheErr != nil {
			common.SysError(cacheErr.Error())
		}
	}
	return
}
