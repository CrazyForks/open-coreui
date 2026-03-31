package migrations

import (
	"context"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
	"github.com/xxnuo/open-coreui/backend/open_webui/migrations/versions"
)

type Definition struct {
	Revision     string
	DownRevision string
	Up           func(context.Context, *dbinternal.Handle) error
}

func Registered() []Definition {
	return []Definition{
		{
			Revision:     versions.V7E5B5DC7342BRevision,
			DownRevision: versions.V7E5B5DC7342BDownRevision,
			Up:           versions.UpgradeV7E5B5DC7342BInit,
		},
		{
			Revision:     versions.CA81BD47C050Revision,
			DownRevision: versions.CA81BD47C050DownRevision,
			Up:           versions.UpgradeCA81BD47C050AddConfigTable,
		},
		{
			Revision:     versions.V3AF16A1C9FB6Revision,
			DownRevision: versions.V3AF16A1C9FB6DownRevision,
			Up:           versions.UpgradeV3AF16A1C9FB6UpdateUserTable,
		},
		{
			Revision:     versions.B10670C03DD5Revision,
			DownRevision: versions.B10670C03DD5DownRevision,
			Up:           versions.UpgradeB10670C03DD5UpdateUserTable,
		},
		{
			Revision:     versions.B2C3D4E5F6A7Revision,
			DownRevision: versions.B2C3D4E5F6A7DownRevision,
			Up:           versions.UpgradeB2C3D4E5F6A7AddScimColumnToUserTable,
		},
	}
}

func Run(ctx context.Context, db *dbinternal.Handle) error {
	for _, migration := range Registered() {
		if err := migration.Up(ctx, db); err != nil {
			return err
		}
	}
	return nil
}
