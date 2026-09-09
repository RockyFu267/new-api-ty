package model

import (
	"time"

	"gorm.io/gorm"
)

type LDAPADIdentity struct {
	Id                int64     `json:"id" gorm:"primaryKey"`
	UserId            int       `json:"user_id" gorm:"not null;uniqueIndex"`
	ObjectGUID        string    `json:"object_guid" gorm:"type:varchar(128);not null;uniqueIndex"`
	Username          string    `json:"username" gorm:"type:varchar(128);not null;index"`
	DistinguishedName string    `json:"distinguished_name" gorm:"type:varchar(512);not null"`
	Disabled          bool      `json:"disabled" gorm:"not null;default:false"`
	LastSyncedAt      time.Time `json:"last_synced_at"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (LDAPADIdentity) TableName() string { return "ldap_ad_identities" }

func GetLDAPADIdentityByGUID(guid string) (*LDAPADIdentity, error) {
	var identity LDAPADIdentity
	if err := DB.Where("object_guid = ?", guid).First(&identity).Error; err != nil {
		return nil, err
	}
	return &identity, nil
}

func EnrichUsersWithAuthSource(users []*User) error {
	if len(users) == 0 {
		return nil
	}
	ids := make([]int, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.Id)
	}
	var identities []LDAPADIdentity
	if err := DB.Select("user_id").Where("user_id IN ?", ids).Find(&identities).Error; err != nil {
		return err
	}
	adUsers := make(map[int]bool, len(identities))
	for _, identity := range identities {
		adUsers[identity.UserId] = true
	}
	for _, user := range users {
		if adUsers[user.Id] {
			if user.Password == "" {
				user.AuthSource = "ad"
			} else {
				user.AuthSource = "local+ad"
			}
		} else {
			user.AuthSource = "local"
		}
	}
	return nil
}

func CreateLDAPADUser(user *User, identity *LDAPADIdentity) error {
	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := user.InsertWithTx(tx, 0); err != nil {
			return err
		}
		identity.UserId = user.Id
		return tx.Create(identity).Error
	}); err != nil {
		return err
	}
	user.FinalizeOAuthUserCreation(0)
	return nil
}
