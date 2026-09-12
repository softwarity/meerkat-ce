package auth

import (
	"context"

	"github.com/softwarity/meerkat/internal/mail"
	"github.com/softwarity/meerkat/internal/store"
)

// The auth package's mails wear the application's identity: the active theme's
// colours and the branding, mapped onto the shared renderer (internal/mail).
// The look itself lives there, so the sign-in mails and the expiry digest never
// drift apart.

// mailPalette reads the active theme's LIGHT palette, complete (the store keeps
// themes complete), falling back to the default so a message is never built
// against half a palette. LIGHT and not the person's scheme: an e-mail dark
// mode is second-guessed by every client, so a mail commits to one look.
func (h *Handler) mailPalette(ctx context.Context) map[string]string {
	t, err := h.st.GetActiveTheme(ctx)
	if err != nil || len(t.Light) == 0 {
		t = store.DefaultTheme()
	}
	return t.Light
}

// buildMail composes a themed message for this account, from the brand the
// pages wear and the active theme's palette.
func (h *Handler) buildMail(ctx context.Context, to string, spec mail.Spec) mail.Message {
	_, brand, _ := h.chrome()
	return mail.Compose(to, mail.Brand{
		AppName: brand.AppName,
		LogoURL: string(brand.LogoURL),
		Meerkat: brand.Meerkat,
	}, h.mailPalette(ctx), spec)
}
