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
			Revision:     versions.V922E7A387820Revision,
			DownRevision: versions.V922E7A387820DownRevision,
			Up:           versions.UpgradeV922E7A387820AddGroupTable,
		},
		{
			Revision:     versions.V37F288994C47Revision,
			DownRevision: versions.V37F288994C47DownRevision,
			Up:           versions.UpgradeV37F288994C47AddGroupMemberTable,
		},
		{
			Revision:     versions.V38D63C18F30FRevision,
			DownRevision: versions.V38D63C18F30FDownRevision,
			Up:           versions.UpgradeV38D63C18F30FAddOAuthSessionTable,
		},
		{
			Revision:     versions.V9F0C9CD09105Revision,
			DownRevision: versions.V9F0C9CD09105DownRevision,
			Up:           versions.UpgradeV9F0C9CD09105AddNoteTable,
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
