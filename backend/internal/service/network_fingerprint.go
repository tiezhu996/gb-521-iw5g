package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	"gorm.io/datatypes"

	"mine-ventilation-network-simulator/backend/internal/constants"
	"mine-ventilation-network-simulator/backend/internal/dto"
	"mine-ventilation-network-simulator/backend/internal/model"
)

// nodeFingerprint 记录发起推演时启用节点参与求解与风险判定的关键参数。
type nodeFingerprint struct {
	ID                 uint    `json:"id"`
	Code               string  `json:"code"`
	NodeType           string  `json:"node_type"`
	ElevationM         float64 `json:"elevation_m"`
	RequiredAirflowM3S float64 `json:"required_airflow_m3s"`
	PressurePa         float64 `json:"pressure_pa"`
}

// edgeFingerprint 记录发起推演时启用巷道的关键参数；停用巷道不进入指纹，
// 之后的启停变化会以新增或移除的形式被识别出来。
type edgeFingerprint struct {
	ID              uint    `json:"id"`
	Code            string  `json:"code"`
	FromNodeID      uint    `json:"from_node_id"`
	ToNodeID        uint    `json:"to_node_id"`
	ResistanceNS2M8 float64 `json:"resistance_ns2m8"`
	AreaM2          float64 `json:"area_m2"`
	MaxVelocityMS   float64 `json:"max_velocity_ms"`
	DoorState       string  `json:"door_state"`
	CriticalPath    bool    `json:"critical_path"`
}

type networkFingerprint struct {
	Hash  string            `json:"hash"`
	Nodes []nodeFingerprint `json:"nodes"`
	Edges []edgeFingerprint `json:"edges"`
}

func buildNetworkFingerprint(nodes []model.VentilationNode, edges []model.AirwayEdge) networkFingerprint {
	fingerprint := networkFingerprint{
		Nodes: make([]nodeFingerprint, 0, len(nodes)),
		Edges: make([]edgeFingerprint, 0, len(edges)),
	}
	for _, node := range nodes {
		fingerprint.Nodes = append(fingerprint.Nodes, nodeFingerprint{
			ID: node.ID, Code: node.Code, NodeType: node.NodeType,
			ElevationM: node.ElevationM, RequiredAirflowM3S: node.RequiredAirflowM3S, PressurePa: node.PressurePa,
		})
	}
	for _, edge := range edges {
		fingerprint.Edges = append(fingerprint.Edges, edgeFingerprint{
			ID: edge.ID, Code: edge.Code, FromNodeID: edge.FromNodeID, ToNodeID: edge.ToNodeID,
			ResistanceNS2M8: edge.ResistanceNS2M8, AreaM2: edge.AreaM2, MaxVelocityMS: edge.MaxVelocityMS,
			DoorState: edge.DoorState, CriticalPath: edge.CriticalPath,
		})
	}
	sort.Slice(fingerprint.Nodes, func(i, j int) bool { return fingerprint.Nodes[i].ID < fingerprint.Nodes[j].ID })
	sort.Slice(fingerprint.Edges, func(i, j int) bool { return fingerprint.Edges[i].ID < fingerprint.Edges[j].ID })
	fingerprint.Hash = fingerprint.computeHash()
	return fingerprint
}

func (fingerprint networkFingerprint) computeHash() string {
	data, err := json.Marshal(struct {
		Nodes []nodeFingerprint `json:"nodes"`
		Edges []edgeFingerprint `json:"edges"`
	}{Nodes: fingerprint.Nodes, Edges: fingerprint.Edges})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// evaluateFreshness 把运行保存的指纹与当前启用节点、巷道参数比照；
// 指纹缺失或无法解析的历史运行标记为 unknown，不按过期处理。
func evaluateFreshness(fingerprintJSON datatypes.JSON, nodes []model.VentilationNode, edges []model.AirwayEdge, checkedAt time.Time) *dto.SimulationFreshness {
	freshness := &dto.SimulationFreshness{
		Status:    string(constants.FreshnessStatusFresh),
		Stale:     false,
		Changes:   []dto.FreshnessChange{},
		CheckedAt: checkedAt,
	}
	if len(fingerprintJSON) == 0 || string(fingerprintJSON) == "null" {
		freshness.Status = string(constants.FreshnessStatusUnknown)
		return freshness
	}
	var baseline networkFingerprint
	if err := json.Unmarshal(fingerprintJSON, &baseline); err != nil || baseline.Hash == "" {
		freshness.Status = string(constants.FreshnessStatusUnknown)
		return freshness
	}
	current := buildNetworkFingerprint(nodes, edges)
	if baseline.Hash == current.Hash {
		return freshness
	}
	freshness.Status = string(constants.FreshnessStatusStale)
	freshness.Stale = true
	freshness.Changes = diffNetworkFingerprint(baseline, current)
	return freshness
}

func diffNetworkFingerprint(baseline, current networkFingerprint) []dto.FreshnessChange {
	changes := make([]dto.FreshnessChange, 0)
	changes = append(changes, diffNodeFingerprints(baseline.Nodes, current.Nodes)...)
	changes = append(changes, diffEdgeFingerprints(baseline.Edges, current.Edges)...)
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].EntityType == changes[j].EntityType {
			return changes[i].EntityID < changes[j].EntityID
		}
		return changes[i].EntityType < changes[j].EntityType
	})
	return changes
}

func diffNodeFingerprints(baseline, current []nodeFingerprint) []dto.FreshnessChange {
	changes := make([]dto.FreshnessChange, 0)
	currentByID := make(map[uint]nodeFingerprint, len(current))
	for _, item := range current {
		currentByID[item.ID] = item
	}
	baselineByID := make(map[uint]nodeFingerprint, len(baseline))
	for _, item := range baseline {
		baselineByID[item.ID] = item
		now, ok := currentByID[item.ID]
		if !ok {
			changes = append(changes, dto.FreshnessChange{EntityType: "ventilation_node", EntityID: item.ID, Code: item.Code, Change: string(constants.FreshnessChangeRemoved)})
			continue
		}
		fields := make([]string, 0)
		if item.Code != now.Code {
			fields = append(fields, "code")
		}
		if item.NodeType != now.NodeType {
			fields = append(fields, "node_type")
		}
		if item.ElevationM != now.ElevationM {
			fields = append(fields, "elevation_m")
		}
		if item.RequiredAirflowM3S != now.RequiredAirflowM3S {
			fields = append(fields, "required_airflow_m3s")
		}
		if item.PressurePa != now.PressurePa {
			fields = append(fields, "pressure_pa")
		}
		if len(fields) > 0 {
			changes = append(changes, dto.FreshnessChange{EntityType: "ventilation_node", EntityID: item.ID, Code: item.Code, Change: string(constants.FreshnessChangeModified), Fields: fields})
		}
	}
	for _, item := range current {
		if _, ok := baselineByID[item.ID]; !ok {
			changes = append(changes, dto.FreshnessChange{EntityType: "ventilation_node", EntityID: item.ID, Code: item.Code, Change: string(constants.FreshnessChangeAdded)})
		}
	}
	return changes
}

func diffEdgeFingerprints(baseline, current []edgeFingerprint) []dto.FreshnessChange {
	changes := make([]dto.FreshnessChange, 0)
	currentByID := make(map[uint]edgeFingerprint, len(current))
	for _, item := range current {
		currentByID[item.ID] = item
	}
	baselineByID := make(map[uint]edgeFingerprint, len(baseline))
	for _, item := range baseline {
		baselineByID[item.ID] = item
		now, ok := currentByID[item.ID]
		if !ok {
			changes = append(changes, dto.FreshnessChange{EntityType: "airway_edge", EntityID: item.ID, Code: item.Code, Change: string(constants.FreshnessChangeRemoved)})
			continue
		}
		fields := make([]string, 0)
		if item.Code != now.Code {
			fields = append(fields, "code")
		}
		if item.FromNodeID != now.FromNodeID {
			fields = append(fields, "from_node_id")
		}
		if item.ToNodeID != now.ToNodeID {
			fields = append(fields, "to_node_id")
		}
		if item.ResistanceNS2M8 != now.ResistanceNS2M8 {
			fields = append(fields, "resistance_ns2m8")
		}
		if item.AreaM2 != now.AreaM2 {
			fields = append(fields, "area_m2")
		}
		if item.MaxVelocityMS != now.MaxVelocityMS {
			fields = append(fields, "max_velocity_ms")
		}
		if item.DoorState != now.DoorState {
			fields = append(fields, "door_state")
		}
		if item.CriticalPath != now.CriticalPath {
			fields = append(fields, "critical_path")
		}
		if len(fields) > 0 {
			changes = append(changes, dto.FreshnessChange{EntityType: "airway_edge", EntityID: item.ID, Code: item.Code, Change: string(constants.FreshnessChangeModified), Fields: fields})
		}
	}
	for _, item := range current {
		if _, ok := baselineByID[item.ID]; !ok {
			changes = append(changes, dto.FreshnessChange{EntityType: "airway_edge", EntityID: item.ID, Code: item.Code, Change: string(constants.FreshnessChangeAdded)})
		}
	}
	return changes
}
