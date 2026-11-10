package model

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	ds "github.com/QuantumNous/new-api/setting/deepseek_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var deepSeekRuleMu sync.Mutex

func GetDeepSeekBillingRules() (ds.Store, error) {
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[ds.OptionKey]
	common.OptionMapRWMutex.RUnlock()
	store := ds.Store{Rules: []ds.Rule{}}
	if raw != "" {
		if err := common.UnmarshalJsonStr(raw, &store); err != nil {
			return store, err
		}
	}
	return store, nil
}

func SaveDeepSeekBillingRule(rule ds.Rule, revision, actor int) (ds.Store, error) {
	deepSeekRuleMu.Lock()
	defer deepSeekRuleMu.Unlock()
	now := time.Now().Unix()
	if rule.EffectiveAt == 0 {
		rule.EffectiveAt = now
	}
	if rule.EffectiveAt < now-5 {
		return ds.Store{}, fmt.Errorf("new rules cannot take effect in the past")
	}
	if err := rule.Validate(); err != nil {
		return ds.Store{}, err
	}
	var result ds.Store
	var raw string
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&Option{Key: ds.OptionKey, Value: `{"revision":0,"rules":[]}`}).Error; err != nil {
			return err
		}
		var option Option
		if err := lockForUpdate(tx).Where(&Option{Key: ds.OptionKey}).First(&option).Error; err != nil {
			return err
		}
		if err := common.UnmarshalJsonStr(option.Value, &result); err != nil {
			return err
		}
		if result.Revision != revision {
			return fmt.Errorf("rules changed; refresh before saving")
		}
		if len(result.Rules) >= 200 {
			return fmt.Errorf("too many pricing versions")
		}
		result.Revision++
		rule.Version = result.Revision
		rule.CreatedBy = actor
		result.Rules = append(result.Rules, rule)
		data, err := common.Marshal(result)
		if err != nil {
			return err
		}
		raw = string(data)
		return tx.Model(&Option{}).Where(&Option{Key: ds.OptionKey}).Update("value", raw).Error
	})
	if err == nil {
		err = updateOptionMap(ds.OptionKey, raw)
	}
	return result, err
}
