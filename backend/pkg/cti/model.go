package cti

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/datatypes"
)

// CTIVulnerability represents a vulnerability pulled from external feeds (NVD, CISA, MITRE).
// It acts as the core Domain model in the CTI package.
type CTIVulnerability struct {
	CVEID           string         `gorm:"primaryKey;column:cve_id;index" json:"cve_id"`
	CVSSV3          float64        `gorm:"column:cvss_v3" json:"cvss_v3"`
	Severity        string         `json:"severity"` // critical | high | medium | low
	Description     string         `json:"description"`
	PublishedAt     time.Time      `gorm:"index" json:"published_at"`
	CISAKnown       bool           `gorm:"column:cisa_known;index" json:"cisa_known"`
	CISADueDate     *time.Time     `gorm:"column:cisa_due_date" json:"cisa_due_date,omitempty"`
	MitreTactics    pq.StringArray `gorm:"type:text[]" json:"mitre_tactics"`
	MitreTechniques pq.StringArray `gorm:"type:text[]" json:"mitre_techniques"`
	AffectedCPE     pq.StringArray `gorm:"type:text[];column:affected_cpe;index:idx_cti_cpe,type:gin" json:"affected_cpe"`
	Remediation     string         `json:"remediation"`
	References      datatypes.JSON `gorm:"type:jsonb" json:"references"`
	LastUpdatedAt   time.Time      `gorm:"column:last_updated_at" json:"last_updated_at"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// TableName returns the PostgreSQL table name for GORM
func (CTIVulnerability) TableName() string {
	return "cti_vulnerabilities"
}
