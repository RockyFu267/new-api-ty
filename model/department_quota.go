package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var ErrDepartmentQuotaUnavailable = errors.New("department quota unavailable")

type DepartmentQuotaRule struct {
	DepartmentId int   `json:"department_id" gorm:"primaryKey;autoIncrement:false"`
	Enabled      bool  `json:"enabled"`
	Daily        int64 `json:"daily"`
	Weekly       int64 `json:"weekly"`
	Monthly      int64 `json:"monthly"`
}

// A reservation belongs to the request's original period and department even
// when it finishes after midnight or the user moves to another department.
type DepartmentQuotaCharge struct {
	Id             int64  `json:"id" gorm:"primaryKey"`
	RequestId      string `json:"request_id" gorm:"type:varchar(128);uniqueIndex;not null"`
	UserId         int    `json:"user_id" gorm:"index:idx_department_charge_user_time,priority:1"`
	DepartmentId   int    `json:"department_id" gorm:"index"`
	DepartmentName string `json:"department_name" gorm:"type:varchar(64)"`
	ModelName      string `json:"model_name" gorm:"type:varchar(255)"`
	CreatedAt      int64  `json:"created_at" gorm:"index:idx_department_charge_user_time,priority:2"`
	Amount         int64  `json:"amount"`
	Status         string `json:"status" gorm:"type:varchar(16)"`
}

type QuotaPeriod struct {
	Start    int64 `json:"start"`
	ResetAt  int64 `json:"reset_at"`
	Limit    int64 `json:"limit"`
	Used     int64 `json:"used"`
	Reserved int64 `json:"reserved"`
}

type DepartmentQuotaUsage struct {
	WalletMonthlyUsed int64       `json:"wallet_monthly_used"`
	UserId            int         `json:"user_id"`
	Username          string      `json:"username"`
	DepartmentId      int         `json:"department_id"`
	DepartmentName    string      `json:"department_name"`
	Configured        bool        `json:"configured"`
	Wallet            int         `json:"wallet"`
	Daily             QuotaPeriod `json:"daily"`
	Weekly            QuotaPeriod `json:"weekly"`
	Monthly           QuotaPeriod `json:"monthly"`
}

func DepartmentQuotaPeriods(now time.Time) [3]QuotaPeriod {
	// Fixed UTC+8 has no daylight-saving ambiguity or dependency on host TZ.
	now = now.In(time.FixedZone("Asia/Shanghai", 8*3600))
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	week := day.AddDate(0, 0, -int(day.Weekday()))
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	return [3]QuotaPeriod{
		{Start: day.Unix(), ResetAt: day.AddDate(0, 0, 1).Unix()},
		{Start: week.Unix(), ResetAt: week.AddDate(0, 0, 7).Unix()},
		{Start: month.Unix(), ResetAt: month.AddDate(0, 1, 0).Unix()},
	}
}

func GetCompanyQuotaUsers(keyword string, departmentId *int, offset, limit int) ([]*User, int64, error) {
	query := DB.Model(&User{})
	if keyword != "" {
		query = query.Where("(username LIKE ? OR display_name LIKE ? OR email LIKE ?)", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if departmentId != nil {
		members := DB.Model(&UserDepartment{}).Select("user_id")
		if *departmentId == 0 {
			query = query.Where("id NOT IN (?)", members)
		} else {
			query = query.Where("id IN (?)", members.Where("department_id = ?", *departmentId))
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	users := make([]*User, 0)
	if err := query.Omit("password", "access_token").Order("id DESC").Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func SaveDepartmentQuotaRule(rule *DepartmentQuotaRule) error {
	for _, amount := range []int64{rule.Daily, rule.Weekly, rule.Monthly} {
		if amount < 0 || amount > int64(common.MaxWalletQuota) {
			return fmt.Errorf("invalid quota amount")
		}
	}
	if rule.Enabled && rule.Daily == 0 && rule.Weekly == 0 && rule.Monthly == 0 {
		return fmt.Errorf("enable at least one quota limit")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var department Department
		if err := lockForUpdate(tx).First(&department, rule.DepartmentId).Error; err != nil {
			return err
		}
		return tx.Save(rule).Error
	})
}

func GetDepartmentQuotaUsage(userId int, now time.Time) (*DepartmentQuotaUsage, error) {
	return departmentQuotaUsage(DB, userId, now)
}

func departmentQuotaUsage(tx *gorm.DB, userId int, now time.Time) (*DepartmentQuotaUsage, error) {
	var user User
	if err := tx.First(&user, userId).Error; err != nil {
		return nil, err
	}
	periods := DepartmentQuotaPeriods(now)
	usage := &DepartmentQuotaUsage{UserId: userId, Username: user.Username, Wallet: user.Quota, Daily: periods[0], Weekly: periods[1], Monthly: periods[2]}
	var member UserDepartment
	err := tx.First(&member, "user_id = ?", userId).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err == nil {
		usage.DepartmentId = member.DepartmentId
		var department Department
		if err := tx.First(&department, member.DepartmentId).Error; err == nil {
			usage.DepartmentName = department.Name
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		var rule DepartmentQuotaRule
		if err := tx.First(&rule, member.DepartmentId).Error; err == nil {
			usage.Configured = rule.Enabled && (rule.Daily > 0 || rule.Weekly > 0 || rule.Monthly > 0)
			usage.Daily.Limit, usage.Weekly.Limit, usage.Monthly.Limit = rule.Daily, rule.Weekly, rule.Monthly
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	for _, period := range []*QuotaPeriod{&usage.Daily, &usage.Weekly, &usage.Monthly} {
		var amounts []struct {
			Status string
			Total  int64
		}
		if err := tx.Model(&DepartmentQuotaCharge{}).Select("status, SUM(amount) AS total").Where("user_id = ? AND created_at >= ? AND created_at < ? AND status IN ?", userId, period.Start, period.ResetAt, []string{"reserved", "settled"}).Group("status").Scan(&amounts).Error; err != nil {
			return nil, err
		}
		for _, amount := range amounts {
			if amount.Status == "reserved" {
				period.Reserved = amount.Total
			} else {
				period.Used = amount.Total
			}
		}
	}
	return usage, nil
}

func ReserveDepartmentQuota(requestId string, userId int, modelName string, amount int64, now time.Time) (*DepartmentQuotaCharge, error) {
	if requestId == "" || amount < 0 || amount > int64(common.MaxWalletQuota) {
		return nil, fmt.Errorf("invalid quota reservation")
	}
	charge := &DepartmentQuotaCharge{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).First(&user, userId).Error; err != nil {
			return err
		}
		err := tx.First(charge, "request_id = ?", requestId).Error
		if err == nil {
			if charge.UserId != userId || charge.Status == "refunded" {
				return fmt.Errorf("invalid reservation reuse")
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		usage, err := departmentQuotaUsage(tx, userId, now)
		if err != nil {
			return err
		}
		if !usage.Configured {
			return ErrDepartmentQuotaUnavailable
		}
		for _, period := range []QuotaPeriod{usage.Daily, usage.Weekly, usage.Monthly} {
			if period.Limit > 0 && (period.Used+period.Reserved >= period.Limit || amount > period.Limit-period.Used-period.Reserved) {
				return ErrDepartmentQuotaUnavailable
			}
		}
		*charge = DepartmentQuotaCharge{RequestId: requestId, UserId: userId, DepartmentId: usage.DepartmentId, DepartmentName: usage.DepartmentName, ModelName: modelName, CreatedAt: now.Unix(), Amount: amount, Status: "reserved"}
		return tx.Create(charge).Error
	})
	return charge, err
}

// Settlement records all actual costs, including in-flight overshoot. Never
// discard a provider charge or refund it just because an estimate was low.
func FinishDepartmentQuota(requestId string, amount int64, refund bool) error {
	if amount < 0 || amount > int64(common.MaxWalletQuota) {
		return fmt.Errorf("invalid settlement")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var charge DepartmentQuotaCharge
		if err := lockForUpdate(tx).First(&charge, "request_id = ?", requestId).Error; err != nil {
			return err
		}
		if charge.Status != "reserved" {
			return nil
		}
		status := "settled"
		if refund {
			status = "refunded"
			amount = 0
		}
		return tx.Model(&charge).Updates(map[string]interface{}{"amount": amount, "status": status}).Error
	})
}

func ResizeDepartmentQuota(requestId string, target int64) error {
	if target < 0 || target > int64(common.MaxWalletQuota) {
		return fmt.Errorf("invalid reservation")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var existing DepartmentQuotaCharge
		if err := tx.First(&existing, "request_id = ?", requestId).Error; err != nil {
			return err
		}
		var user User
		if err := lockForUpdate(tx).First(&user, existing.UserId).Error; err != nil {
			return err
		}
		if err := lockForUpdate(tx).First(&existing, existing.Id).Error; err != nil {
			return err
		}
		if existing.Status != "reserved" {
			return fmt.Errorf("reservation is closed")
		}
		if target > existing.Amount {
			usage, err := departmentQuotaUsage(tx, existing.UserId, time.Unix(existing.CreatedAt, 0))
			if err != nil {
				return err
			}
			for _, period := range []QuotaPeriod{usage.Daily, usage.Weekly, usage.Monthly} {
				if period.Limit > 0 && target-existing.Amount > period.Limit-period.Used-period.Reserved {
					return ErrDepartmentQuotaUnavailable
				}
			}
		}
		return tx.Model(&existing).Update("amount", target).Error
	})
}
