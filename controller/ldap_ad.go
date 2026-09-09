package controller

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/go-ldap/ldap/v3"
	"gorm.io/gorm"
)

type ldapADUser struct {
	ObjectGUID        string `json:"object_guid"`
	Username          string `json:"username"`
	DisplayName       string `json:"display_name"`
	Email             string `json:"email"`
	DistinguishedName string `json:"distinguished_name"`
	Disabled          bool   `json:"disabled"`
	SyncStatus        string `json:"sync_status,omitempty"`
	UserId            int    `json:"user_id,omitempty"`
}

func ldapADConnect() (*ldap.Conn, error) {
	settings := system_setting.GetLDAPADSettings()
	if strings.TrimSpace(settings.URL) == "" || strings.TrimSpace(settings.BindDN) == "" || settings.BindSecret == "" {
		return nil, errors.New("AD 域控配置不完整")
	}
	conn, err := ldap.DialURL(strings.TrimSpace(settings.URL), ldap.DialWithDialer(&net.Dialer{Timeout: 8 * time.Second}))
	if err != nil {
		return nil, fmt.Errorf("连接域控失败: %w", err)
	}
	conn.SetTimeout(10 * time.Second)
	if err = conn.Bind(settings.BindDN, settings.BindSecret); err != nil {
		conn.Close()
		return nil, fmt.Errorf("域控绑定失败: %w", err)
	}
	return conn, nil
}

func ldapADFilter(username string) string {
	settings := system_setting.GetLDAPADSettings()
	filter := strings.TrimSpace(settings.UserFilter)
	if filter == "" {
		filter = "(&(objectClass=user)(objectCategory=person)(sAMAccountName={username}))"
	}
	value := ldap.EscapeFilter(username)
	if username == "*" {
		value = "*"
	}
	return strings.ReplaceAll(filter, "{username}", value)
}

func ldapADSearch(conn *ldap.Conn, username string, sizeLimit int) ([]ldapADUser, error) {
	settings := system_setting.GetLDAPADSettings()
	request := ldap.NewSearchRequest(settings.SearchBaseDN(), ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		sizeLimit, 10, false, ldapADFilter(username),
		[]string{"objectGUID", settings.LoginAttr, "displayName", "mail", "distinguishedName", "userAccountControl"}, nil)
	result, err := conn.Search(request)
	if err != nil {
		return nil, err
	}
	users := make([]ldapADUser, 0, len(result.Entries))
	for _, entry := range result.Entries {
		rawGUID := entry.GetRawAttributeValue("objectGUID")
		if len(rawGUID) == 0 {
			continue
		}
		usernameValue := strings.TrimSpace(entry.GetAttributeValue(settings.LoginAttr))
		if usernameValue == "" {
			continue
		}
		control, _ := entry.GetAttributeValue("userAccountControl"), 0
		disabled := false
		if control != "" {
			var value int
			_, _ = fmt.Sscanf(control, "%d", &value)
			disabled = value&2 != 0
		}
		users = append(users, ldapADUser{ObjectGUID: base64.RawURLEncoding.EncodeToString(rawGUID), Username: usernameValue,
			DisplayName: entry.GetAttributeValue("displayName"), Email: model.NormalizeEmail(entry.GetAttributeValue("mail")),
			DistinguishedName: entry.DN, Disabled: disabled})
	}
	return users, nil
}

func annotateLDAPADUsers(users []ldapADUser) {
	for i := range users {
		if identity, err := model.GetLDAPADIdentityByGUID(users[i].ObjectGUID); err == nil {
			users[i].SyncStatus, users[i].UserId = "synced", identity.UserId
			continue
		}
		var count int64
		query := model.DB.Model(&model.User{}).Where("username = ?", users[i].Username)
		if users[i].Email != "" {
			query = model.DB.Model(&model.User{}).Where("username = ? OR LOWER(email) = ?", users[i].Username, users[i].Email)
		}
		query.Count(&count)
		if count > 0 {
			users[i].SyncStatus = "conflict"
		} else {
			users[i].SyncStatus = "new"
		}
	}
}

func TestLDAPADConnection(c *gin.Context) {
	conn, err := ldapADConnect()
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	defer conn.Close()
	common.ApiSuccess(c, gin.H{"connected": true})
}

func PreviewLDAPADUsers(c *gin.Context) {
	conn, err := ldapADConnect()
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	defer conn.Close()
	users, err := ldapADSearch(conn, "*", 500)
	if err != nil {
		common.ApiErrorMsg(c, "读取 AD 用户失败: "+err.Error())
		return
	}
	annotateLDAPADUsers(users)
	common.ApiSuccess(c, users)
}

func makeLDAPADUser(entry ldapADUser) (*model.User, *model.LDAPADIdentity, error) {
	if utf8.RuneCountInString(entry.Username) > model.UserNameMaxLength {
		return nil, nil, errors.New("用户名超过系统长度限制")
	}
	displayName := []rune(strings.TrimSpace(entry.DisplayName))
	if len(displayName) > 20 {
		displayName = displayName[:20]
	}
	user := &model.User{Username: entry.Username, DisplayName: string(displayName), Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Email: entry.Email}
	identity := &model.LDAPADIdentity{ObjectGUID: entry.ObjectGUID, Username: entry.Username, DistinguishedName: entry.DistinguishedName, Disabled: entry.Disabled, LastSyncedAt: time.Now()}
	return user, identity, nil
}

func SyncLDAPADUsers(c *gin.Context) {
	conn, err := ldapADConnect()
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	defer conn.Close()
	users, err := ldapADSearch(conn, "*", 500)
	if err != nil {
		common.ApiErrorMsg(c, "读取 AD 用户失败: "+err.Error())
		return
	}
	created, skipped := 0, 0
	for _, entry := range users {
		if entry.Disabled {
			skipped++
			continue
		}
		if _, err := model.GetLDAPADIdentityByGUID(entry.ObjectGUID); err == nil {
			continue
		}
		user, identity, err := makeLDAPADUser(entry)
		if err != nil {
			skipped++
			continue
		}
		if err = model.CreateLDAPADUser(user, identity); err != nil {
			skipped++
			continue
		}
		created++
	}
	common.ApiSuccess(c, gin.H{"created": created, "skipped": skipped, "total": len(users)})
}

type ldapADLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func LDAPADLogin(c *gin.Context) {
	if !system_setting.GetLDAPADSettings().Enabled {
		common.ApiErrorMsg(c, "域账号登录未启用")
		return
	}
	var request ldapADLoginRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || strings.TrimSpace(request.Username) == "" || request.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请输入域账号和密码"})
		return
	}
	conn, err := ldapADConnect()
	if err != nil {
		common.ApiErrorMsg(c, "域账号登录失败")
		return
	}
	users, err := ldapADSearch(conn, strings.TrimSpace(request.Username), 2)
	conn.Close()
	if err != nil || len(users) != 1 || users[0].Disabled {
		common.ApiErrorMsg(c, "域账号或密码错误")
		return
	}
	entry := users[0]
	userConn, err := ldap.DialURL(system_setting.GetLDAPADSettings().URL, ldap.DialWithDialer(&net.Dialer{Timeout: 8 * time.Second}))
	if err != nil {
		common.ApiErrorMsg(c, "域账号登录失败")
		return
	}
	defer userConn.Close()
	if err = userConn.Bind(entry.DistinguishedName, request.Password); err != nil {
		common.ApiErrorMsg(c, "域账号或密码错误")
		return
	}
	identity, err := model.GetLDAPADIdentityByGUID(entry.ObjectGUID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		user, newIdentity, createErr := makeLDAPADUser(entry)
		if createErr != nil || model.CreateLDAPADUser(user, newIdentity) != nil {
			common.ApiErrorMsg(c, "域账号尚未同步或与本地用户冲突")
			return
		}
		identity = newIdentity
	} else if err != nil {
		common.ApiErrorMsg(c, "域账号登录失败")
		return
	}
	user, err := model.GetUserById(identity.UserId, false)
	if err != nil || user.Status != common.UserStatusEnabled {
		common.ApiErrorMsg(c, "用户不可用")
		return
	}
	setupLogin(user, c)
}
