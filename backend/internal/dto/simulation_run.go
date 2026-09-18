package dto

import (
	"time"

	"mine-ventilation-network-simulator/backend/internal/model"
)

type StartSimulationRequest struct {
	ScenarioID uint `json:"scenario_id" binding:"required"`
}

type ConfirmRisksRequest struct {
	Note string `json:"note" binding:"required,min=4,max=500"`
}

type SimulationListQuery struct {
	Page       int    `form:"page" binding:"omitempty,gte=1"`
	PageSize   int    `form:"page_size" binding:"omitempty,gte=1,lte=100"`
	Status     string `form:"status"`
	ScenarioID uint   `form:"scenario_id"`
}

type NetworkIssue struct {
	Code       string `json:"code"`
	EntityType string `json:"entity_type"`
	EntityID   uint   `json:"entity_id"`
	Message    string `json:"message"`
	Severity   string `json:"severity"`
}

type RiskEvidence struct {
	RuleCode    string  `json:"rule_code"`
	Level       string  `json:"level"`
	EntityType  string  `json:"entity_type"`
	EntityID    uint    `json:"entity_id"`
	Evidence    float64 `json:"evidence"`
	Threshold   float64 `json:"threshold"`
	Unit        string  `json:"unit"`
	Description string  `json:"description"`
}

// FreshnessChange 指出推演指纹与当前网络参数不一致的具体对象。
type FreshnessChange struct {
	EntityType string   `json:"entity_type"`
	EntityID   uint     `json:"entity_id"`
	Code       string   `json:"code"`
	Change     string   `json:"change"`
	Fields     []string `json:"fields,omitempty"`
}

// SimulationFreshness 是发起推演时保存的网络参数指纹与当前参数的比照结果。
type SimulationFreshness struct {
	Status    string            `json:"status"`
	Stale     bool              `json:"stale"`
	Changes   []FreshnessChange `json:"changes"`
	CheckedAt time.Time         `json:"checked_at"`
}

// SimulationRunView 在推演记录上叠加实时新鲜度校验结果，历史结果本身不被修改。
type SimulationRunView struct {
	model.SimulationRun
	Freshness *SimulationFreshness `json:"freshness"`
}
