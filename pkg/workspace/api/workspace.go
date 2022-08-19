package api

import (
	"time"

	"github.com/google/uuid"
)

type CreateWorkspaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Tier        string `json:"tier"`
}

type CreateWorkspaceResponse struct {
	ID string `json:"id"`
}

type ChangeWorkspaceOwnershipRequest struct {
	NewOwnerUserID string `json:"newOwnerUserID"`
}

type WorkspaceResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	OwnerId     uuid.UUID `json:"ownerId"`
	Tier        string    `json:"tier"`
	URI         string    `json:"uri"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}

type WorkspaceLimits struct {
	MaxUsers       int64 `json:"maxUsers"`
	MaxConnections int64 `json:"maxConnections"`
	MaxResources   int64 `json:"maxResources"`
}

type WorkspaceLimitsUsage struct {
	CurrentUsers       int64 `json:"currentUsers"`
	CurrentConnections int64 `json:"currentConnections"`
	CurrentResources   int64 `json:"currentResources"`

	MaxUsers       int64 `json:"maxUsers"`
	MaxConnections int64 `json:"maxConnections"`
	MaxResources   int64 `json:"maxResources"`
}
