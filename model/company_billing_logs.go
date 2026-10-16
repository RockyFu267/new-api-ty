package model

import (
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

func GetCompanyBillingLogs(source string, userId int, modelName string, offset, limit int, stripAdmin bool) ([]*Log, int64, error) {
	if source == "department_quota" {
		return departmentBillingPage(userId, modelName, offset, limit, stripAdmin)
	}
	query := LOG_DB.Model(&Log{}).Where("type = ?", LogTypeConsume)
	query, err := applyCompanyBillingSource(query, source)
	if err != nil {
		return nil, 0, err
	}
	if userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	if modelName != "" {
		query = query.Where("model_name = ?", modelName)
	}
	return companyLogPage(query, offset, limit, stripAdmin)
}

// Company statements use the authoritative ledger, not optional consumption
// logs. Token/model details are enriched from those logs when available.
func departmentBillingPage(userId int, modelName string, offset, limit int, stripAdmin bool) ([]*Log, int64, error) {
	query := DB.Model(&DepartmentQuotaCharge{}).Where("status = ?", "settled")
	if userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	if modelName != "" {
		query = query.Where("model_name = ?", modelName)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var charges []DepartmentQuotaCharge
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&charges).Error; err != nil {
		return nil, 0, err
	}
	requestIds := make([]string, 0, len(charges))
	userIds := make([]int, 0, len(charges))
	for _, charge := range charges {
		requestIds = append(requestIds, charge.RequestId)
		userIds = append(userIds, charge.UserId)
	}
	var details []Log
	if len(requestIds) > 0 {
		if err := LOG_DB.Where("request_id IN ? AND type = ?", requestIds, LogTypeConsume).Find(&details).Error; err != nil {
			common.SysError(err.Error())
		}
	}
	byRequest := make(map[string]Log)
	for _, detail := range details {
		byRequest[detail.RequestId] = detail
	}
	var users []User
	if len(userIds) > 0 {
		if err := DB.Unscoped().Select("id", "username").Where("id IN ?", userIds).Find(&users).Error; err != nil {
			return nil, 0, err
		}
	}
	names := make(map[int]string)
	for _, user := range users {
		names[user.Id] = user.Username
	}
	logs := make([]*Log, 0, len(charges))
	for _, charge := range charges {
		detail, found := byRequest[charge.RequestId]
		detail.Id = int(charge.Id)
		detail.UserId = charge.UserId
		detail.Username = names[charge.UserId]
		detail.RequestId = charge.RequestId
		detail.Type = LogTypeConsume
		detail.ModelName = charge.ModelName
		detail.Quota = int(charge.Amount)
		detail.CreatedAt = charge.CreatedAt
		other, _ := common.StrToMap(detail.Other)
		if other == nil {
			other = make(map[string]interface{})
		}
		other["billing_source"] = "department_quota"
		other["department_id"] = charge.DepartmentId
		other["department_name"] = charge.DepartmentName
		other["token_data_available"] = found
		detail.Other = common.MapToJsonStr(other)
		logs = append(logs, &detail)
	}
	if stripAdmin {
		formatUserLogs(logs, offset)
	}
	return logs, total, nil
}

func GetWalletAdjustmentLogs(keyword, action, start, end string, offset, limit int) ([]*Log, int64, error) {
	query := LOG_DB.Model(&Log{}).Where("type = ? AND other LIKE ?", LogTypeManage, `%"action":"user.quota_%`)
	if keyword != "" {
		query = query.Where("other LIKE ?", "%"+keyword+"%")
	}
	if action != "" {
		if action != "add" && action != "subtract" && action != "override" {
			return nil, 0, fmt.Errorf("invalid operation")
		}
		query = query.Where("other LIKE ?", `%"action":"user.quota_`+action+`"%`)
	}
	for _, filter := range []struct {
		Value     string
		Condition string
	}{{start, "created_at >= ?"}, {end, "created_at <= ?"}} {
		if filter.Value != "" {
			stamp, err := strconv.ParseInt(filter.Value, 10, 64)
			if err != nil {
				return nil, 0, err
			}
			query = query.Where(filter.Condition, stamp)
		}
	}
	return companyLogPage(query, offset, limit, false)
}

func companyLogPage(query *gorm.DB, offset, limit int, stripAdmin bool) ([]*Log, int64, error) {
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	logs := make([]*Log, 0)
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	if stripAdmin {
		formatUserLogs(logs, offset)
	}
	return logs, total, nil
}

func applyCompanyBillingSource(query *gorm.DB, source string) (*gorm.DB, error) {
	switch source {
	case "":
		return query, nil
	case "department_quota", "subscription":
		return query.Where("type = ? AND other LIKE ?", LogTypeConsume, `%"billing_source":"`+source+`"%`), nil
	case "wallet":
		return query.Where("type = ? AND (other IS NULL OR (other NOT LIKE ? AND other NOT LIKE ?))", LogTypeConsume, `%"billing_source":"department_quota"%`, `%"billing_source":"subscription"%`), nil
	default:
		return nil, fmt.Errorf("invalid billing source")
	}
}

func WalletMonthConsumption(userId int, start, end int64) (int64, error) {
	query, err := applyCompanyBillingSource(LOG_DB.Model(&Log{}), "wallet")
	if err != nil {
		return 0, err
	}
	var amount int64
	err = query.Where("user_id = ? AND created_at >= ? AND created_at < ?", userId, start, end).Select("COALESCE(SUM(quota),0)").Scan(&amount).Error
	return amount, err
}
