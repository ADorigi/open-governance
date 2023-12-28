package entity

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kaytu-io/kaytu-engine/services/integration/model"
	"github.com/kaytu-io/kaytu-util/pkg/source"
)

type InvalidConnectionLifecycleStateError struct {
	c ConnectionLifecycleState
}

func (err InvalidConnectionLifecycleStateError) Error() string {
	return fmt.Sprintf("invalid connection lifecycle state: %s", err.c)
}

type ConnectionLifecycleState string

const (
	ConnectionLifecycleStateOnboard    ConnectionLifecycleState = "ONBOARD"
	ConnectionLifecycleStateDisabled   ConnectionLifecycleState = "DISABLED"
	ConnectionLifecycleStateDiscovered ConnectionLifecycleState = "DISCOVERED"
	ConnectionLifecycleStateInProgress ConnectionLifecycleState = "IN_PROGRESS"
	ConnectionLifecycleStateArchived   ConnectionLifecycleState = "ARCHIVED"
)

func (c ConnectionLifecycleState) Validate() error {
	switch c {
	case ConnectionLifecycleStateInProgress, ConnectionLifecycleStateOnboard, ConnectionLifecycleStateDisabled:
		return nil
	default:
		return InvalidConnectionLifecycleStateError{c}
	}
}

type ConnectionCountRequest struct {
	ConnectorsNames []string                  `json:"connectors" example:"Azure"`
	State           *ConnectionLifecycleState `json:"state" example:"enabled"`
}

type Connection struct {
	ID                   uuid.UUID                       `json:"id" example:"8e0f8e7a-1b1c-4e6f-b7e4-9c6af9d2b1c8"`
	ConnectionID         string                          `json:"providerConnectionID" example:"8e0f8e7a-1b1c-4e6f-b7e4-9c6af9d2b1c8"`
	ConnectionName       string                          `json:"providerConnectionName" example:"example-connection"`
	Email                string                          `json:"email" example:"johndoe@example.com"`
	Connector            source.Type                     `json:"connector" example:"Azure"`
	Description          string                          `json:"description" example:"This is an example connection"`
	OnboardDate          time.Time                       `json:"onboardDate" example:"2023-05-07T00:00:00Z"`
	AssetDiscoveryMethod source.AssetDiscoveryMethodType `json:"assetDiscoveryMethod" example:"scheduled"`

	CredentialID   string         `json:"credentialID" example:"7r6123ac-ca1c-434f-b1a3-91w2w9d277c8"`
	CredentialName *string        `json:"credentialName,omitempty"`
	CredentialType CredentialType `json:"credentialType" example:"manual"`
	Credential     Credential     `json:"credential,omitempty"`

	LifecycleState ConnectionLifecycleState `json:"lifecycleState" example:"enabled"`

	HealthState         source.HealthStatus `json:"healthState" example:"healthy"`
	LastHealthCheckTime time.Time           `json:"lastHealthCheckTime" example:"2023-05-07T00:00:00Z"`
	HealthReason        *string             `json:"healthReason,omitempty"`
	AssetDiscovery      *bool               `json:"assetDiscovery,omitempty"`
	SpendDiscovery      *bool               `json:"spendDiscovery,omitempty"`

	LastInventory        *time.Time `json:"lastInventory" example:"2023-05-07T00:00:00Z"`
	Cost                 *float64   `json:"cost" example:"1000.00" minimum:"0" maximum:"10000000"`
	DailyCostAtStartTime *float64   `json:"dailyCostAtStartTime" example:"1000.00" minimum:"0" maximum:"10000000"`
	DailyCostAtEndTime   *float64   `json:"dailyCostAtEndTime" example:"1000.00"  minimum:"0" maximum:"10000000"`
	ResourceCount        *int       `json:"resourceCount" example:"100" minimum:"0" maximum:"1000000"`
	OldResourceCount     *int       `json:"oldResourceCount" example:"100" minimum:"0" maximum:"1000000"`

	Metadata           map[string]any `json:"metadata"`
	DescribeJobRunning bool
}

func (c Connection) IsEnabled() bool {
	if c.LifecycleState == ConnectionLifecycleStateOnboard ||
		c.LifecycleState == ConnectionLifecycleStateInProgress {
		return true
	}
	return false
}

func (c Connection) IsDiscovered() bool {
	return c.LifecycleState == ConnectionLifecycleStateDiscovered
}

type ChangeConnectionLifecycleStateRequest struct {
	State ConnectionLifecycleState `json:"state"`
}

type ListConnectionsResponse []Connection

type ListConnectionsSummaryResponse struct {
	ConnectionCount       int     `json:"connectionCount" example:"10" minimum:"0" maximum:"1000"`
	TotalCost             float64 `json:"totalCost" example:"1000.00" minimum:"0" maximum:"10000000"`
	TotalResourceCount    int     `json:"totalResourceCount" example:"100" minimum:"0" maximum:"1000000"`
	TotalOldResourceCount int     `json:"totalOldResourceCount" example:"100" minimum:"0" maximum:"1000000"`
	TotalUnhealthyCount   int     `json:"totalUnhealthyCount" example:"10" minimum:"0" maximum:"100"`

	TotalDisabledCount   int          `json:"totalDisabledCount" example:"10" minimum:"0" maximum:"100"`
	TotalDiscoveredCount int          `json:"totalDiscoveredCount" example:"10" minimum:"0" maximum:"100"`
	TotalOnboardedCount  int          `json:"totalOnboardedCount" example:"10" minimum:"0" maximum:"100"` // Also includes in-progress
	TotalArchivedCount   int          `json:"totalArchivedCount" example:"10" minimum:"0" maximum:"100"`
	Connections          []Connection `json:"connections"`
}

type ChangeConnectionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Email       string `json:"email"`
}

type CreateConnectionResponse struct {
	ID uuid.UUID `json:"id"`
}

// NewConnection creates API compatible connection from model connection.
func NewConnection(s model.Connection) Connection {
	metadata := make(map[string]any)
	if len(metadata) > 0 {
		_ = json.Unmarshal(s.Metadata, &metadata)
	}

	conn := Connection{
		ID:                   s.ID,
		ConnectionID:         s.SourceId,
		ConnectionName:       s.Name,
		Email:                s.Email,
		Connector:            s.Type,
		Description:          s.Description,
		CredentialID:         s.CredentialID.String(),
		CredentialName:       s.Credential.Name,
		CredentialType:       NewCredentialType(s.Credential.CredentialType),
		OnboardDate:          s.CreatedAt,
		HealthState:          s.HealthState,
		LifecycleState:       ConnectionLifecycleState(s.LifecycleState),
		AssetDiscoveryMethod: s.AssetDiscoveryMethod,
		LastHealthCheckTime:  s.LastHealthCheckTime,
		HealthReason:         s.HealthReason,
		Metadata:             metadata,
		AssetDiscovery:       s.AssetDiscovery,
		SpendDiscovery:       s.SpendDiscovery,

		ResourceCount: nil,
		Cost:          nil,
		LastInventory: nil,
	}
	return conn
}
