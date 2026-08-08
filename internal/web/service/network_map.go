package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"github.com/oschwald/maxminddb-golang/v2"
)

type NetworkMapNode struct {
	ID          int     `json:"id"`
	Guid        string  `json:"guid"`
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	LatencyMs   int     `json:"latencyMs"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	OwnerUserID *int    `json:"ownerUserId,omitempty"`
}

type NetworkMapConnection struct {
	NodeID      int     `json:"nodeId"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	LastSeen    int64   `json:"lastSeen"`
	ActiveCount int     `json:"activeCount"`
	Source      string  `json:"source"`
}

type NetworkMapPayload struct {
	GeoReady    bool                   `json:"geoReady"`
	GPSReady    bool                   `json:"gpsReady"`
	GeneratedAt int64                  `json:"generatedAt"`
	Nodes       []NetworkMapNode       `json:"nodes"`
	Connections []NetworkMapConnection `json:"connections"`
}

type CustomerLocationInput struct {
	Latitude  float64 `json:"latitude" form:"latitude"`
	Longitude float64 `json:"longitude" form:"longitude"`
	AccuracyM float64 `json:"accuracyM" form:"accuracyM"`
}

func validateCustomerLocation(input CustomerLocationInput) error {
	if math.IsNaN(input.Latitude) || math.IsInf(input.Latitude, 0) ||
		input.Latitude < -90 || input.Latitude > 90 {
		return fmt.Errorf("latitude is out of range")
	}
	if math.IsNaN(input.Longitude) || math.IsInf(input.Longitude, 0) ||
		input.Longitude < -180 || input.Longitude > 180 {
		return fmt.Errorf("longitude is out of range")
	}
	if math.IsNaN(input.AccuracyM) || math.IsInf(input.AccuracyM, 0) ||
		input.AccuracyM < 0 || input.AccuracyM > 100000 {
		return fmt.Errorf("accuracy is out of range")
	}
	return nil
}

func SaveCustomerLocation(userID int, input CustomerLocationInput) (*model.CustomerLocation, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid customer")
	}
	if err := validateCustomerLocation(input); err != nil {
		return nil, err
	}
	location := &model.CustomerLocation{
		UserID: userID, Latitude: input.Latitude, Longitude: input.Longitude,
		AccuracyM: input.AccuracyM, UpdatedAt: time.Now().Unix(),
	}
	err := database.GetDB().Where("user_id = ?", userID).
		Assign(map[string]any{
			"latitude": input.Latitude, "longitude": input.Longitude,
			"accuracy_m": input.AccuracyM, "updated_at": location.UpdatedAt,
		}).
		FirstOrCreate(location).Error
	return location, err
}

func ClearCustomerLocation(userID int) error {
	if userID <= 0 {
		return fmt.Errorf("invalid customer")
	}
	return database.GetDB().Where("user_id = ?", userID).
		Delete(&model.CustomerLocation{}).Error
}

// SaveCustomerIPLocation records a coarse city-level location from a
// customer's login IP. It deliberately accepts only Chinese addresses: when a
// customer reaches the landing server through an overseas relay, that relay
// must never be mistaken for the customer's location. Browser-authorised GPS
// remains stored separately and always takes precedence on the map.
func SaveCustomerIPLocation(userID int, rawIP string) (*model.CustomerLocation, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid customer")
	}
	reader, err := networkGeoReader.get()
	if err != nil {
		return nil, err
	}
	lat, lon, ok := lookupChinaLocation(reader, rawIP)
	if !ok {
		return nil, nil
	}
	now := time.Now().Unix()
	location := &model.CustomerLocation{UserID: userID}
	err = database.GetDB().Where("user_id = ?", userID).
		Assign(map[string]any{
			"ip_latitude": lat, "ip_longitude": lon, "ip_updated_at": now,
		}).FirstOrCreate(location).Error
	return location, err
}

type geoCityRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
}

type geoReaderCache struct {
	mu      sync.Mutex
	path    string
	modTime time.Time
	reader  *maxminddb.Reader
}

var networkGeoReader geoReaderCache

func geoDatabasePath() string {
	if path := strings.TrimSpace(os.Getenv("NOVAPANEL_GEOIP_DB")); path != "" {
		return path
	}
	return filepath.Join("/etc", "x-ui", "GeoLite2-City.mmdb")
}

func (c *geoReaderCache) get() (*maxminddb.Reader, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	path := geoDatabasePath()
	info, statErr := os.Stat(path)
	if statErr != nil {
		return nil, statErr
	}
	if c.reader != nil && c.path == path && c.modTime.Equal(info.ModTime()) {
		return c.reader, nil
	}
	reader, err := maxminddb.Open(path)
	if err != nil {
		return nil, err
	}
	if c.reader != nil {
		_ = c.reader.Close()
	}
	c.reader = reader
	c.path = path
	c.modTime = info.ModTime()
	return reader, nil
}

func lookupChinaLocation(reader *maxminddb.Reader, rawIP string) (float64, float64, bool) {
	ip, err := netip.ParseAddr(strings.TrimSpace(rawIP))
	if err != nil || !ip.IsGlobalUnicast() {
		return 0, 0, false
	}
	var record geoCityRecord
	if err := reader.Lookup(ip.Unmap()).Decode(&record); err != nil {
		return 0, 0, false
	}
	if record.Country.ISOCode != "CN" || record.Location.Latitude == 0 || record.Location.Longitude == 0 {
		return 0, 0, false
	}
	// City databases are approximate. Rounding also prevents the API from
	// exposing a misleadingly precise customer location.
	lat := float64(int(record.Location.Latitude*10)) / 10
	lon := float64(int(record.Location.Longitude*10)) / 10
	return lat, lon, true
}

type connectionBucket struct {
	nodeID   int
	lat      float64
	lon      float64
	lastSeen int64
	ips      map[string]struct{}
}

func GetNetworkMap(ownerUserID *int) (*NetworkMapPayload, error) {
	db := database.GetDB()
	query := db.Model(&model.Node{}).Where("enable = ?", true).Order("id asc")
	if ownerUserID != nil {
		query = query.Where("owner_user_id = ?", *ownerUserID)
	}
	var nodes []*model.Node
	if err := query.Find(&nodes).Error; err != nil {
		return nil, err
	}

	payload := &NetworkMapPayload{
		GeneratedAt: time.Now().Unix(),
		Nodes:       make([]NetworkMapNode, 0, len(nodes)),
		Connections: []NetworkMapConnection{},
	}
	guidToNode := make(map[string]int, len(nodes))
	nodeOwners := make(map[int]int, len(nodes))
	ownerIDs := make([]int, 0, len(nodes))
	seenOwners := make(map[int]struct{}, len(nodes))
	for _, node := range nodes {
		attributionKey := effectiveNodeKey(node)
		payload.Nodes = append(payload.Nodes, NetworkMapNode{
			ID: node.Id, Guid: attributionKey, Name: node.Name, Status: node.Status,
			LatencyMs: node.LatencyMs, Latitude: node.Latitude,
			Longitude: node.Longitude, OwnerUserID: node.OwnerUserID,
		})
		if attributionKey != "" {
			guidToNode[attributionKey] = node.Id
		}
		if node.OwnerUserID != nil {
			nodeOwners[node.Id] = *node.OwnerUserID
			if _, exists := seenOwners[*node.OwnerUserID]; !exists {
				seenOwners[*node.OwnerUserID] = struct{}{}
				ownerIDs = append(ownerIDs, *node.OwnerUserID)
			}
		}
	}
	const preciseLocationWindow = int64(24 * 60 * 60)
	preciseLocations := make(map[int]model.CustomerLocation, len(ownerIDs))
	if len(ownerIDs) > 0 {
		var locations []model.CustomerLocation
		if err := db.Where("user_id IN ? AND updated_at >= ?", ownerIDs,
			payload.GeneratedAt-preciseLocationWindow).Find(&locations).Error; err != nil {
			return nil, err
		}
		for _, location := range locations {
			preciseLocations[location.UserID] = location
		}
	}
	payload.GPSReady = len(preciseLocations) > 0
	for _, node := range nodes {
		if node.OwnerUserID == nil {
			continue
		}
		location, ok := preciseLocations[*node.OwnerUserID]
		if !ok {
			continue
		}
		payload.Connections = append(payload.Connections, NetworkMapConnection{
			NodeID: node.Id, Latitude: location.Latitude, Longitude: location.Longitude,
			LastSeen: location.UpdatedAt, ActiveCount: 0, Source: "gps",
		})
	}
	// A successful customer-panel login provides a consent-free, city-level
	// fallback. It is used only while no fresh browser-authorised GPS position
	// exists, and it is associated with every node owned by that customer.
	var loginLocations []model.CustomerLocation
	if len(ownerIDs) > 0 {
		if err := db.Where("user_id IN ? AND ip_updated_at >= ?", ownerIDs,
			payload.GeneratedAt-preciseLocationWindow).Find(&loginLocations).Error; err != nil {
			return nil, err
		}
	}
	loginByOwner := make(map[int]model.CustomerLocation, len(loginLocations))
	for _, location := range loginLocations {
		loginByOwner[location.UserID] = location
	}
	for _, node := range nodes {
		if node.OwnerUserID == nil {
			continue
		}
		if _, hasGPS := preciseLocations[*node.OwnerUserID]; hasGPS {
			continue
		}
		location, ok := loginByOwner[*node.OwnerUserID]
		if !ok || (location.IPLatitude == 0 && location.IPLongitude == 0) {
			continue
		}
		payload.Connections = append(payload.Connections, NetworkMapConnection{
			NodeID: node.Id, Latitude: location.IPLatitude, Longitude: location.IPLongitude,
			LastSeen: location.IPUpdatedAt, ActiveCount: 0, Source: "login-ip",
		})
	}
	if len(guidToNode) == 0 {
		return payload, nil
	}

	reader, err := networkGeoReader.get()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return payload, nil
		}
		return payload, nil
	}
	payload.GeoReady = true

	var rows []model.NodeClientIp
	if err := db.Where("node_guid IN ?", mapKeys(guidToNode)).Find(&rows).Error; err != nil {
		return nil, err
	}
	const recentWindow = int64(15 * 60)
	buckets := map[string]*connectionBucket{}
	for _, row := range rows {
		nodeID, ok := guidToNode[row.NodeGuid]
		if !ok {
			continue
		}
		var entries []model.ClientIpEntry
		if err := json.Unmarshal([]byte(row.Ips), &entries); err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.Timestamp < payload.GeneratedAt-recentWindow {
				continue
			}
			lat, lon, ok := lookupChinaLocation(reader, entry.IP)
			if !ok {
				continue
			}
			if ownerID, exists := nodeOwners[nodeID]; exists {
				if _, hasGPS := preciseLocations[ownerID]; hasGPS {
					continue
				}
			}
			key := strings.Join([]string{
				row.NodeGuid,
				formatCoordinate(lat),
				formatCoordinate(lon),
			}, ":")
			bucket := buckets[key]
			if bucket == nil {
				bucket = &connectionBucket{
					nodeID: nodeID, lat: lat, lon: lon,
					ips: make(map[string]struct{}),
				}
				buckets[key] = bucket
			}
			bucket.ips[entry.IP] = struct{}{}
			if entry.Timestamp > bucket.lastSeen {
				bucket.lastSeen = entry.Timestamp
			}
		}
	}
	for _, bucket := range buckets {
		payload.Connections = append(payload.Connections, NetworkMapConnection{
			NodeID: bucket.nodeID, Latitude: bucket.lat, Longitude: bucket.lon,
			LastSeen: bucket.lastSeen, ActiveCount: len(bucket.ips), Source: "ip",
		})
	}
	sort.Slice(payload.Connections, func(i, j int) bool {
		return payload.Connections[i].LastSeen > payload.Connections[j].LastSeen
	})
	return payload, nil
}

func mapKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func formatCoordinate(value float64) string {
	return strings.TrimRight(strings.TrimRight(
		strconv.FormatFloat(value, 'f', 1, 64), "0"), ".")
}
