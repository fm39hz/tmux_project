package tmux

import (
	"context"

	"github.com/fm39hz/gotomux/internal/model"
)

type Connector interface {
	ListLive(ctx context.Context) ([]LiveSession, error)
	Has(ctx context.Context, name string) bool
	CurrentSession(ctx context.Context) string
	CurrentSessionPath(ctx context.Context) string
	Kill(ctx context.Context, name string) error
	Freeze(ctx context.Context, name string) (*model.Session, error)
	Load(ctx context.Context, p *model.Session) error
	Connect(ctx context.Context, name, cwd string) error
	ConnectPreset(ctx context.Context, p *model.Session) error
}
