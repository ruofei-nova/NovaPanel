package service

import (
	"encoding/json"
	"errors"
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
}

type NetworkMapPayload struct {
	GeoReady    bool                   `json:"geoReady"`
	GeneratedAt int64                  `json:"generatedAt"`
	Nodes       []NetworkMapNode       `json:"nodes"`
	Connections []NetworkMapConnection `json:"connections"`
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
	mu     sync.Mutex
	path   string
	reader *maxminddb.Reader
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
	if c.reader != nil && c.path == path {
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
	for _, node := range nodes {
		payload.Nodes = append(payload.Nodes, NetworkMapNode{
			ID: node.Id, Guid: node.Guid, Name: node.Name, Status: node.Status,
			LatencyMs: node.LatencyMs, Latitude: node.Latitude,
			Longitude: node.Longitude, OwnerUserID: node.OwnerUserID,
		})
		if node.Guid != "" {
			guidToNode[node.Guid] = node.Id
		}
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
			LastSeen: bucket.lastSeen, ActiveCount: len(bucket.ips),
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
