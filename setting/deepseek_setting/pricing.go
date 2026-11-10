package deepseek_setting

import (
	"fmt"
	"math"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
)

const Model = "deepseek-v4.1-flash"
const OptionKey = "DeepSeekBillingRules"

var Location = time.FixedZone("Asia/Shanghai", 8*3600)

type Rates struct {
	Input  float64 `json:"input"`
	Cache  float64 `json:"cache"`
	Output float64 `json:"output"`
}
type Window struct {
	Start int `json:"start"`
	End   int `json:"end"`
}
type Rule struct {
	Version     int      `json:"version"`
	CreatedBy   int      `json:"created_by"`
	Enabled     bool     `json:"enabled"`
	EffectiveAt int64    `json:"effective_at"`
	FX          float64  `json:"fx"` // CNY per USD; independent from display/payment FX.
	Peak        Rates    `json:"peak"`
	OffPeak     Rates    `json:"off_peak"`
	Windows     []Window `json:"windows"`
	Holidays    []string `json:"holidays"`
}
type Store struct {
	Revision int    `json:"revision"`
	Rules    []Rule `json:"rules"`
}
type Snapshot struct {
	Rule      Rule  `json:"rule"`
	StartedAt int64 `json:"started_at"`
	Peak      bool  `json:"peak"`
	Rates     Rates `json:"rates"`
}
type Charge struct {
	QuotaRounding string   `json:"quota_rounding"`
	Snapshot      Snapshot `json:"snapshot"`
	Input         int64    `json:"input"`
	Cache         int64    `json:"cache"`
	Output        int64    `json:"output"`
	CNY           float64  `json:"cny"`
	USD           float64  `json:"usd"`
	Quota         int      `json:"quota"`
}

func DefaultRule() Rule {
	return Rule{FX: 7.3, Peak: Rates{2, .04, 8}, OffPeak: Rates{1, .02, 4}, Windows: []Window{{540, 720}, {840, 1080}}, Holidays: []string{"2026-09-25", "2026-09-26", "2026-09-27", "2026-10-01", "2026-10-02", "2026-10-03", "2026-10-04", "2026-10-05", "2026-10-06", "2026-10-07"}}
}

func (r Rule) Validate() error {
	if r.EffectiveAt <= 0 || r.FX < .01 || r.FX > 1000 || math.IsNaN(r.FX) || math.IsInf(r.FX, 0) {
		return fmt.Errorf("invalid effective time or CNY/USD exchange rate")
	}
	for _, rates := range []Rates{r.Peak, r.OffPeak} {
		for _, n := range []float64{rates.Input, rates.Cache, rates.Output} {
			if n <= 0 || n > 100000 || math.IsNaN(n) || math.IsInf(n, 0) {
				return fmt.Errorf("unit prices must be positive finite numbers")
			}
		}
	}
	if len(r.Windows) == 0 || len(r.Windows) > 8 || len(r.Holidays) > 1000 {
		return fmt.Errorf("invalid peak windows or holiday count")
	}
	last := 0
	for _, w := range r.Windows {
		if w.Start < last || w.Start < 0 || w.End > w.Start+1440 || w.End > 1440 || w.End <= w.Start {
			return fmt.Errorf("peak windows must be ordered and non-overlapping")
		}
		last = w.End
	}
	for _, day := range r.Holidays {
		if _, err := time.Parse("2006-01-02", day); err != nil {
			return fmt.Errorf("invalid holiday date: %s", day)
		}
	}
	return nil
}

func (s Store) Select(model string, at time.Time) *Snapshot {
	if model != Model {
		return nil
	}
	var selected *Rule
	for _, r := range s.Rules {
		if r.EffectiveAt <= at.Unix() && (selected == nil || r.EffectiveAt > selected.EffectiveAt || r.EffectiveAt == selected.EffectiveAt && r.Version > selected.Version) {
			copy := r
			selected = &copy
		}
	}
	if selected == nil || !selected.Enabled {
		return nil
	}
	local := at.In(Location)
	peak := false
	if local.Weekday() != time.Saturday && local.Weekday() != time.Sunday {
		minute := local.Hour()*60 + local.Minute()
		for _, w := range selected.Windows {
			if minute >= w.Start && minute < w.End {
				peak = true
			}
		}
		for _, day := range selected.Holidays {
			if day == local.Format("2006-01-02") {
				peak = false
			}
		}
	}
	rates := selected.OffPeak
	if peak {
		rates = selected.Peak
	}
	return &Snapshot{Rule: *selected, StartedAt: at.Unix(), Peak: peak, Rates: rates}
}

func (s Snapshot) Calculate(input, cache, output int64) (Charge, error) {
	if input < 0 || cache < 0 || output < 0 || cache > input {
		return Charge{}, fmt.Errorf("invalid DeepSeek token usage")
	}
	if err := s.Rule.Validate(); err != nil {
		return Charge{}, err
	}
	cny := decimal.NewFromInt(input - cache).Mul(decimal.NewFromFloat(s.Rates.Input)).Add(decimal.NewFromInt(cache).Mul(decimal.NewFromFloat(s.Rates.Cache))).Add(decimal.NewFromInt(output).Mul(decimal.NewFromFloat(s.Rates.Output))).Div(decimal.NewFromInt(1000000))
	usd := cny.Div(decimal.NewFromFloat(s.Rule.FX))
	quota, err := common.QuotaFromDecimalStrict(usd.Mul(decimal.NewFromFloat(common.QuotaPerUnit)))
	if err != nil {
		return Charge{}, err
	}
	if cny.IsPositive() && quota == 0 {
		quota = 1
	}
	return Charge{QuotaRounding: "nearest_quota_unit", Snapshot: s, Input: input, Cache: cache, Output: output, CNY: cny.InexactFloat64(), USD: usd.InexactFloat64(), Quota: quota}, nil
}
