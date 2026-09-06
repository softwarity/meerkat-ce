import { provideLivewire } from '@softwarity/livewire';
import { ApplicationConfig } from '@angular/core';
import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { MAT_BUTTON_TOGGLE_DEFAULT_OPTIONS } from '@angular/material/button-toggle';
import { MAT_FORM_FIELD_DEFAULT_OPTIONS } from '@angular/material/form-field';
import { provideRouter, withComponentInputBinding } from '@angular/router';
import { provideStore } from '@softwarity/store';
import { routes } from './app.routes';
import { authInterceptor } from './auth.interceptor';

// No animations provider: since v20.2 the animations package is deprecated -
// Material animates natively (animate.enter/animate.leave, plain CSS).
export const appConfig: ApplicationConfig = {
  providers: [
    provideRouter(routes, withComponentInputBinding()),
    // One socket for the whole console (OBS-01 and whatever follows it): a
    // screen registers a topic rather than opening a connection of its own.
    // Root-relative, so it goes out on the origin the console was served from
    // and carries the session cookie that authorises it.
    provideLivewire({ path: '/api/live' }),
    provideHttpClient(withInterceptors([authInterceptor])),
    // Persist UI table preferences (sort, filters) in browser storage.
    provideStore(),
    // Every mat-form-field is outline by default (no per-field appearance).
    { provide: MAT_FORM_FIELD_DEFAULT_OPTIONS, useValue: { appearance: 'outline' } },
    // Button toggles never show a selection checkmark - it shifts the label and
    // breaks the layout; selection is conveyed by the fill alone.
    {
      provide: MAT_BUTTON_TOGGLE_DEFAULT_OPTIONS,
      useValue: { hideSingleSelectionIndicator: true, hideMultipleSelectionIndicator: true },
    },
  ],
};
