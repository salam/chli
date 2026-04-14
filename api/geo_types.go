package api

import "encoding/json"

// GeoSearchResponse wraps the geo.admin.ch SearchServer JSON shape.
type GeoSearchResponse struct {
	Results []GeoSearchResult `json:"results"`
}

type GeoSearchResult struct {
	ID     json.Number `json:"id"`
	Weight int         `json:"weight,omitempty"`
	Attrs  GeoAttrs    `json:"attrs"`
}

type GeoAttrs struct {
	Label       string  `json:"label,omitempty"`
	Detail      string  `json:"detail,omitempty"`
	Layer       string  `json:"layer,omitempty"`
	FeatureID   string  `json:"featureId,omitempty"`
	Origin      string  `json:"origin,omitempty"`
	Lat         float64 `json:"lat,omitempty"`
	Lon         float64 `json:"lon,omitempty"`
	Y           float64 `json:"y,omitempty"`
	X           float64 `json:"x,omitempty"`
	Geom_ST_Box *string `json:"geom_st_box2d,omitempty"`
	Zoomlevel   int     `json:"zoomlevel,omitempty"`
}

// GeoLayersResponse from /rest/services/api/MapServer
type GeoLayersResponse struct {
	Layers []GeoLayer `json:"layers"`
}

type GeoLayer struct {
	LayerBodID   string `json:"layerBodId"`
	FullyQualifiedLayerBodID string `json:"fullyQualifiedLayerBodId,omitempty"`
	Type         string `json:"type,omitempty"`
	Category     string `json:"category,omitempty"`
	Attribution  string `json:"attribution,omitempty"`
	MaxResolution float64 `json:"maxResolution,omitempty"`
	MinResolution float64 `json:"minResolution,omitempty"`
}

// GeoIdentifyResponse from /rest/services/api/MapServer/identify
type GeoIdentifyResponse struct {
	Results []GeoIdentifyResult `json:"results"`
}

type GeoIdentifyResult struct {
	LayerBodID string          `json:"layerBodId"`
	LayerName  string          `json:"layerName"`
	FeatureID  json.RawMessage `json:"featureId"`
	ID         json.RawMessage `json:"id"`
	Attributes map[string]any  `json:"attributes"`
}
