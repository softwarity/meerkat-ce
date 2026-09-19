import { Component } from '@angular/core';
import { RouterOutlet } from '@angular/router';

// The application is one outlet: the shell under a language, and a page under
// the shell. Everything the reader sees comes from content/ - see
// site/shell.component.ts.
@Component({
  selector: 'app-root',
  imports: [RouterOutlet],
  template: '<router-outlet />',
})
export class AppComponent {}
