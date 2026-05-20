// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)


package app

import (
	"errors"
	"io"
)

// Close will be called in graceful shutdown in start cmd
func (app *Evmos) Close() error {
	err := app.BaseApp.Close()

	if cms, ok := app.CommitMultiStore().(io.Closer); ok {
		return errors.Join(err, cms.Close())
	}

	return err
}
