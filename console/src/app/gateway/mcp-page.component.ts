import { Component, computed, inject, LOCALE_ID, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatIconModule } from '@angular/material/icon';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { DateTime } from 'luxon';
import { RouterLink } from '@angular/router';
import { AdminToken, ApiService } from '../api.service';
import { DialogsService } from '../shared/dialogs.service';

// One client's instructions. The commands differ enough between agents that a
// single JSON block would be a riddle: three of these take the same flags and
// the fourth takes none of them, and two write their configuration in
// different files under different keys.
interface AgentClient {
  id: string;
  label: string;
  // command is what to run; json is what to write when there is no command.
  command: (url: string) => string;
  // withToken is the same, for a client that cannot do the browser flow.
  withToken: (url: string) => string;
}

const CLIENTS: AgentClient[] = [
  {
    id: 'claude',
    label: 'Claude Code',
    command: (url) => `claude mcp add --transport http meerkat ${url}`,
    withToken: (url) =>
      `claude mcp add --transport http meerkat ${url} --header "Authorization: Bearer $MEERKAT_MCP_TOKEN"`,
  },
  {
    id: 'gemini',
    label: 'Gemini CLI',
    command: (url) => `gemini mcp add --transport http meerkat ${url}`,
    withToken: (url) =>
      `gemini mcp add --transport http meerkat ${url} --header "Authorization: Bearer $MEERKAT_MCP_TOKEN"`,
  },
  {
    id: 'kimi',
    label: 'Kimi CLI',
    command: (url) => `kimi mcp add --transport http meerkat ${url}`,
    withToken: (url) =>
      `kimi mcp add --transport http meerkat ${url} --header "Authorization: Bearer $MEERKAT_MCP_TOKEN"`,
  },
  {
    id: 'codex',
    label: 'Codex CLI',
    // Codex keeps its own shape, and its own second step: the login is not
    // implicit there.
    command: (url) => `codex mcp add meerkat --url ${url}\ncodex mcp login meerkat`,
    withToken: (url) =>
      `codex mcp add meerkat --url ${url} --bearer-token-env-var MEERKAT_MCP_TOKEN`,
  },
  {
    id: 'json',
    label: 'Other (JSON)',
    command: (url) =>
      JSON.stringify({ mcpServers: { meerkat: { type: 'http', url } } }, null, 2),
    withToken: (url) =>
      JSON.stringify(
        {
          mcpServers: {
            meerkat: { type: 'http', url, headers: { Authorization: 'Bearer ${MEERKAT_MCP_TOKEN}' } },
          },
        },
        null,
        2,
      ),
  },
];

// The MCP section (MCP-01/07): where an agent is connected, and where the ones
// already connected are seen and cut off.
//
// It sits on its own rather than under Access tokens, because the two answer
// different questions. Access tokens is "a key for the REST API"; this is "let
// my assistant work on this gateway" - and the whole point of the OAuth flow
// is that answering the second one produces no key at all.
@Component({
  selector: 'app-mcp-page',
  imports: [
    MatButtonModule,
    MatButtonToggleModule,
    MatExpansionModule,
    MatIconModule,
    MatSlideToggleModule,
    MatTooltipModule,
    LoadingIndicatorComponent,
    RouterLink,
  ],
  styleUrl: './mcp-page.component.scss',
  templateUrl: './mcp-page.component.html',
})
export class McpPageComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  private readonly dialogs = inject(DialogsService);
  private readonly locale = inject(LOCALE_ID);

  protected readonly loading = signal(true);
  protected readonly enabled = signal(false);
  protected readonly agents = signal<AdminToken[]>([]);
  protected readonly copied = signal('');

  protected readonly clients = CLIENTS;
  protected readonly client = signal(CLIENTS[0]);

  // Whatever address this console is reached at IS the control plane's
  // address: there is nothing else to ask anybody for.
  protected readonly url = window.location.origin + '/mcp';

  protected readonly command = computed(() => this.client().command(this.url));
  protected readonly tokenCommand = computed(() => this.client().withToken(this.url));

  constructor() {
    this.api.getAgentEndpoint().subscribe({
      next: (s) => this.enabled.set(s.enabled),
      error: () => undefined,
    });
    this.load();
  }

  private load(): void {
    this.api.listAdminTokens().subscribe({
      next: (tokens) => {
        // A connection is a token that belongs to a registered agent. The
        // others are keys somebody minted by hand, and they live next door.
        this.agents.set(tokens.filter((t) => !!t.clientId));
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }

  protected setEnabled(enabled: boolean): void {
    this.api.setAgentEndpoint(enabled).subscribe({
      next: () => this.enabled.set(enabled),
      error: (err) => {
        this.snack.open(errMsg(err), undefined, { duration: 4000 });
        this.enabled.set(!enabled);
      },
    });
  }

  protected pick(client: AgentClient): void {
    this.client.set(client);
    this.copied.set('');
  }

  protected copy(text: string, what: string): void {
    void navigator.clipboard.writeText(text).then(() => this.copied.set(what));
  }

  protected async revoke(t: AdminToken): Promise<void> {
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Disconnect_NAME:Disconnect "${t.clientName || t.name}:NAME:"?`,
      confirmLabel: $localize`:@@Disconnect:Disconnect`,
      danger: true,
    });
    if (!ok) return;
    this.api.revokeAdminToken(t.id).subscribe({
      next: () => this.agents.update((list) => list.filter((x) => x.id !== t.id)),
      error: (err) => this.snack.open(errMsg(err), undefined, { duration: 4000 }),
    });
  }

  protected perimeter(t: AdminToken): string {
    const what =
      t.scope === 'readonly'
        ? $localize`:@@Read_only:Read only`
        : $localize`:@@Read_and_change:Read and change`;
    switch (t.domain) {
      case 'gateway':
        return what + ' - ' + $localize`:@@The_routing_plane:The routing plane`;
      case 'app':
        return what + ' - ' + $localize`:@@The_applications_identity:The application's identity`;
      default:
        return what;
    }
  }

  protected lastUsed(ts: number): string {
    if (!ts) return $localize`:@@never_used:never used`;
    const rel = DateTime.fromSeconds(ts).reconfigure({ locale: this.locale }).toRelative() ?? '';
    return $localize`:@@last_used_REL:last used ${rel}:REL:`;
  }
}

function errMsg(err: unknown): string {
  const e = err as { error?: { error?: string } };
  return typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Request_failed:Request failed`;
}
