package httpserver

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gitlab.com/keibiengine/keibi-engine/pkg/auth/api"
)

const (
	XKeibiWorkspaceNameHeader = "X-Keibi-WorkspaceName"
	XKeibiUserIDHeader        = "X-Keibi-UserId"
	XKeibiUserRoleHeader      = "X-Keibi-UserRole"
)

func AuthorizeHandler(h echo.HandlerFunc, minRole api.Role) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		if err := RequireMinRole(ctx, minRole); err != nil {
			return err
		}

		return h(ctx)
	}
}

func RequireMinRole(ctx echo.Context, minRole api.Role) error {
	if !hasAccess(GetUserRole(ctx), minRole) {
		return echo.NewHTTPError(http.StatusForbidden, "missing required permission")
	}

	return nil
}

func GetWorkspaceName(ctx echo.Context) string {
	name := ctx.Request().Header.Get(XKeibiWorkspaceNameHeader)
	if strings.TrimSpace(name) == "" {
		panic(fmt.Errorf("header %s is missing", XKeibiWorkspaceNameHeader))
	}

	return name
}

func GetUserRole(ctx echo.Context) api.Role {
	role := ctx.Request().Header.Get(XKeibiUserRoleHeader)
	if strings.TrimSpace(role) == "" {
		panic(fmt.Errorf("header %s is missing", XKeibiUserRoleHeader))
	}

	return api.Role(role)
}

func GetUserID(ctx echo.Context) uuid.UUID {
	id := ctx.Request().Header.Get(XKeibiUserIDHeader)
	if strings.TrimSpace(id) == "" {
		panic(fmt.Errorf("header %s is missing", XKeibiUserIDHeader))
	}

	u, err := uuid.Parse(id)
	if err != nil {
		panic(err)
	}

	return u
}

func roleToPriority(role api.Role) int {
	switch role {
	case api.ViewerRole:
		return 0
	case api.EditorRole:
		return 1
	case api.AdminRole:
		return 2
	default:
		panic("unsupported role: " + role)
	}
}

func hasAccess(currRole, minRole api.Role) bool {
	return roleToPriority(currRole) >= roleToPriority(minRole)
}
