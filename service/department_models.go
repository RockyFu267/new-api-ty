package service

import (
	"fmt"

	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// Snapshot access once per request. New requests re-read the database, including
// on other nodes; retries/in-flight work keep the permissions they started with.
func CheckDepartmentModelAccess(c *gin.Context, name string) error {
	value, _ := c.Get("department_model_access")
	access, ok := value.(model.DepartmentModelAccess)
	if !ok {
		var err error
		access, err = model.GetUserDepartmentModelAccess(c.GetInt("id"))
		if err != nil {
			return fmt.Errorf("%s", i18n.T(c, "department_models.unavailable"))
		}
		c.Set("department_model_access", access)
	}
	if access.DepartmentID == 0 {
		return fmt.Errorf("%s", i18n.T(c, "department_models.no_department"))
	}
	if !access.Allows(name) {
		return fmt.Errorf("%s", i18n.T(c, "department_models.forbidden", map[string]any{"Model": name}))
	}
	return nil
}
