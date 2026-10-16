package service

import (
	"time"

	"github.com/QuantumNous/new-api/model"
)

const BillingSourceDepartment = "department_quota"

type DepartmentFunding struct {
	RequestId string
	UserId    int
	ModelName string
	Consumed  int64
	Charge    *model.DepartmentQuotaCharge
}

func (d *DepartmentFunding) Source() string { return BillingSourceDepartment }

func (d *DepartmentFunding) PreConsume(amount int) error {
	charge, err := model.ReserveDepartmentQuota(d.RequestId, d.UserId, d.ModelName, int64(amount), time.Now())
	if err != nil {
		return err
	}
	d.Charge = charge
	d.Consumed = charge.Amount
	return nil
}

func (d *DepartmentFunding) Settle(delta int) error {
	return model.FinishDepartmentQuota(d.RequestId, d.Consumed+int64(delta), false)
}

func (d *DepartmentFunding) Refund() error {
	return refundWithRetry(func() error { return model.FinishDepartmentQuota(d.RequestId, 0, true) })
}
