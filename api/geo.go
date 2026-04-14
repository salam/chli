package api

import (
	"fmt"
	"net/url"
	"time"
)

const (
	geoBaseURL  = "https://api3.geo.admin.ch"
	geoCacheTTL = 24 * time.Hour
)

// GeoSearch queries the geo.admin.ch SearchServer.
//
// typ: "locations" (addresses/places), "layers" (map layers), "featuresearch"
// (attribute search in layers), or "" for locations by default.
func (c *Client) GeoSearch(query, typ string, limit int) (*GeoSearchResponse, error) {
	if typ == "" {
		typ = "locations"
	}
	if limit <= 0 {
		limit = 15
	}
	q := url.Values{}
	q.Set("searchText", query)
	q.Set("type", typ)
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("sr", "4326")

	var out GeoSearchResponse
	err := c.DoJSONWithTTL(geoBaseURL, "/rest/services/api/SearchServer?"+q.Encode(), &out, geoCacheTTL)
	return &out, err
}

// GeoLayers lists available map layers.
func (c *Client) GeoLayers() (*GeoLayersResponse, error) {
	var out GeoLayersResponse
	err := c.DoJSONWithTTL(geoBaseURL, "/rest/services/api/MapServer", &out, geoCacheTTL)
	return &out, err
}

// GeoIdentify identifies features at a WGS84 coordinate on the given layers.
// layers is a comma-separated list of layerBodIds (e.g. "ch.kantone.cantonal-boundaries").
func (c *Client) GeoIdentify(lon, lat float64, layers string) (*GeoIdentifyResponse, error) {
	if layers == "" {
		layers = "ch.bfs.gebaeude_wohnungs_register,ch.kantone.cantonal-boundaries,ch.swisstopo-vd.ortschaftenverzeichnis_plz"
	}
	// geometry for identify: "lon,lat" in EPSG:4326 with imageDisplay=0,0,96 and mapExtent tiny
	q := url.Values{}
	q.Set("geometry", fmt.Sprintf("%f,%f", lon, lat))
	q.Set("geometryType", "esriGeometryPoint")
	q.Set("sr", "4326")
	// Small extent around the point; units are degrees in 4326.
	extent := fmt.Sprintf("%f,%f,%f,%f", lon-0.0005, lat-0.0005, lon+0.0005, lat+0.0005)
	q.Set("mapExtent", extent)
	q.Set("imageDisplay", "96,96,96")
	q.Set("tolerance", "10")
	q.Set("layers", "all:"+layers)
	q.Set("returnGeometry", "false")

	var out GeoIdentifyResponse
	err := c.DoJSONWithTTL(geoBaseURL, "/rest/services/api/MapServer/identify?"+q.Encode(), &out, geoCacheTTL)
	return &out, err
}
