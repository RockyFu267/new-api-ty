package controller

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLDAPADFilterEscapesLoginInputAndSupportsPreview(t *testing.T) {
	settings := system_setting.GetLDAPADSettings()
	original := *settings
	t.Cleanup(func() { *settings = original })
	settings.UserFilter = "(&(objectClass=user)(sAMAccountName={username}))"

	filter := ldapADFilter("alice*)(|(objectClass=*))")
	assert.False(t, strings.Contains(filter, "(|(objectClass=*))"))
	assert.Contains(t, filter, `\2a`)
	assert.Contains(t, ldapADFilter("*"), "(sAMAccountName=*)")
}

func TestMakeLDAPADUserAlwaysCreatesOrdinaryUser(t *testing.T) {
	user, identity, err := makeLDAPADUser(ldapADUser{
		ObjectGUID: "guid", Username: "domain-user", DisplayName: "Domain User", DistinguishedName: "CN=Domain User,DC=example,DC=com",
	})
	require.NoError(t, err)
	assert.Equal(t, common.RoleCommonUser, user.Role)
	assert.Equal(t, common.UserStatusEnabled, user.Status)
	assert.Empty(t, user.Password)
	assert.Equal(t, "guid", identity.ObjectGUID)
}

func TestMakeLDAPADUserRejectsUsernameBeyondExistingLimit(t *testing.T) {
	_, _, err := makeLDAPADUser(ldapADUser{Username: strings.Repeat("a", modelUserNameLimitForTest())})
	require.Error(t, err)
}

func modelUserNameLimitForTest() int { return 21 }
