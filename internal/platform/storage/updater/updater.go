package updater

import (
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/storage"
)

type Updater interface {
	Start() error
	Shutdown() error
	Load() (storage.DBOpenAPILoader, error)
	Find(rctx *router.Context, schemaID int, method, path string) (router.Handler, error)
}
