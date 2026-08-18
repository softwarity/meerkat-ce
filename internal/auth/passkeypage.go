package auth

import (
	"net/http"
	"time"
)

var profilePasskeysPage = flowPage("profile-passkeys", profilePasskeysBody)

type profilePasskeysData struct {
	flowChrome
	Passkeys []passkeyView
}

// The passkeys, on their own page. They were a section of Security, under the
// links, where a list that grows with every browser someone signs in from read
// as an afterthought - and the button to add one sat below the fold.
const profilePasskeysBody = `    <style>
      .pk-name { flex: 1; font-size: .88rem; text-align: start; }
      .pk-date { font-family: var(--mk-mono); font-size: .68rem; color: var(--mk-on-surface-variant); }
      .pk-this {
        font-family: var(--mk-mono); font-size: .58rem; letter-spacing: .12em;
        text-transform: uppercase; color: var(--mk-primary);
        padding: 2px 8px; border-radius: 999px;
        background: color-mix(in srgb, var(--mk-primary) 12%, transparent);
      }
      /* small round button, same family as the scheme switch pills */
      .pk-x {
        margin: 0; padding: 0; width: 28px; height: 28px;
        border: 1px solid transparent; border-radius: 50%;
        background: none; box-shadow: none; color: var(--mk-on-surface-variant);
        cursor: pointer; display: grid; place-items: center;
      }
      .pk-x svg { display: block; }
      .pk-x:hover {
        color: var(--mk-error); border-color: var(--mk-outline);
        background: var(--mk-surface-container); filter: none; box-shadow: none;
      }
      .pk-x:active { transform: none; }
      .hint-line { margin: 0; font-size: .74rem; color: var(--mk-on-surface-variant); }
    </style>
    <div class="panel">
      <h2>{{.T.passkeys}}</h2>
      <div class="rows">
      {{range .Passkeys}}
      <div class="row">
        <span class="pk-name">{{.Label}}</span>
        {{if .Current}}<span class="pk-this">{{$.T.thisBrowser}}</span>{{end}}
        <span class="pk-date">{{.Created}}</span>
        <form method="post" action="/profile/passkeys/delete">
          <input type="hidden" name="id" value="{{.ID}}">
          <button class="pk-x" type="submit" title="{{$.T.passkeyRemove}}" aria-label="{{$.T.passkeyRemove}}"><svg viewBox="0 -960 960 960" width="15" height="15" fill="currentColor" aria-hidden="true"><path d="m256-200-56-56 224-224-224-224 56-56 224 224 224-224 56 56-224 224 224 224-56 56-224-224-224 224Z"/></svg></button>
        </form>
      </div>
      {{end}}
      </div>
      <button type="button" class="choice" id="pk-add">{{.T.addPasskey}}</button>
      <p class="hint-line" id="pk-msg" hidden></p>
    </div>
    <script>
    (() => {
      const b64d = (s) => Uint8Array.from(atob(s.replace(/-/g, '+').replace(/_/g, '/')), (c) => c.charCodeAt(0));
      const b64e = (b) => btoa(String.fromCharCode(...new Uint8Array(b))).replace(/[+]/g, '-').replace(/[/]/g, '_').replace(/=+$/, '');
      const btn = document.getElementById('pk-add');
      const msg = document.getElementById('pk-msg');
      if (!btn) return;
      if (!window.PublicKeyCredential) { btn.disabled = true; return; }
      btn.addEventListener('click', async () => {
        msg.hidden = true;
        try {
          const start = await fetch('/profile/passkeys/register/start', { method: 'POST' }).then((r) => r.json());
          const pk = start.options.publicKey;
          pk.challenge = b64d(pk.challenge);
          pk.user.id = b64d(pk.user.id);
          (pk.excludeCredentials || []).forEach((c) => { c.id = b64d(c.id); });
          const cred = await navigator.credentials.create({ publicKey: pk });
          const body = {
            id: cred.id, rawId: b64e(cred.rawId), type: cred.type,
            response: {
              attestationObject: b64e(cred.response.attestationObject),
              clientDataJSON: b64e(cred.response.clientDataJSON),
            },
          };
          const fin = await fetch('/profile/passkeys/register/finish?challenge=' + start.challenge, {
            method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body),
          });
          if (!fin.ok) throw new Error(await fin.text());
          location.reload();
        } catch (e) {
          msg.textContent = String((e && e.message) || e);
          msg.hidden = false;
        }
      });
    })();
    </script>
    <p class="back"><a href="/profile/security">{{.T.back}}</a></p>
`

// showPasskeys renders the passkey page, reached from Security. Closed when
// the gateway-wide policy is off, exactly as the ceremonies are.
func (h *Handler) showPasskeys(w http.ResponseWriter, r *http.Request) {
	sess, err := h.sm.Resolve(r.Context(), r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if sess.Pending != "" {
		http.Redirect(w, r, "/"+sess.Pending, http.StatusSeeOther)
		return
	}
	if !h.passkeysOffered(r.Context()) {
		http.NotFound(w, r)
		return
	}
	data := profilePasskeysData{flowChrome: listChrome(h.flowData(r, "titlePasskeys"))}
	current := ""
	if c, err := r.Cookie(passkeyCookie); err == nil {
		current = c.Value
	}
	if keys, err := h.st.ListPasskeys(r.Context(), sess.UserID); err == nil {
		for _, k := range keys {
			data.Passkeys = append(data.Passkeys, passkeyView{
				ID: k.ID, Label: k.Label,
				Created: time.Unix(k.CreatedAt, 0).Format("2006-01-02"),
				Current: k.ID == current,
			})
		}
	}
	writeFlow(w, profilePasskeysPage, data, http.StatusOK)
}
