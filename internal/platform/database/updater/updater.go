package updater

import (
	"github.com/wallarm/api-firewall/internal/platform/database"
	"github.com/wallarm/api-firewall/internal/platform/router"
)

type Updater interface {
	Start() error
	Shutdown() error
	Load() (database.DBOpenAPILoader, error)
	Find(rctx *router.Context, schemaID int, method, path string) (router.Handler, error)
}
