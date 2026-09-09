package system_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

type LDAPADSettings struct {
	Enabled           bool   `json:"enabled"`
	URL               string `json:"url"`
	BaseDN            string `json:"base_dn"`
	BindDN            string `json:"bind_dn"`
	BindSecret        string `json:"bind_secret"`
	LoginAttr         string `json:"login_attr"`
	UserFilter        string `json:"user_filter"`
	AllowedUserBaseDN string `json:"allowed_user_base_dn"`
}

var defaultLDAPADSettings = LDAPADSettings{
	LoginAttr:  "sAMAccountName",
	UserFilter: "(&(objectClass=user)(objectCategory=person)(sAMAccountName={username}))",
}

func init() {
	config.GlobalConfig.Register("ldap_ad", &defaultLDAPADSettings)
}

func GetLDAPADSettings() *LDAPADSettings { return &defaultLDAPADSettings }

func (s *LDAPADSettings) SearchBaseDN() string {
	if value := strings.TrimSpace(s.AllowedUserBaseDN); value != "" {
		return value
	}
	return strings.TrimSpace(s.BaseDN)
}
