import { Component, computed, inject } from '@angular/core';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatCardModule } from '@angular/material/card';
import { MatIconModule } from '@angular/material/icon';
import { EeLockComponent } from '../../shared/ee-lock.component';
import { BuiltInPagesScope } from './built-in-pages.scope';

// One entry of the gallery: the name the server knows, what to call it, and
// the sentence that says when someone would want it.
interface LayoutChoice {
  name: string;
  label: string;
  what: string;
  sided: boolean;
}

// The Layout tab (PAGE-02): pick an arrangement for the pages this gateway
// serves.
//
// The thumbnails are CSS mock-ups, not live frames. Five real previews to
// CHOOSE from would be five times the weight for a decision taken on shape
// alone - and the real one is right there on the other half of the screen,
// showing the candidate the moment it is clicked.
@Component({
  selector: 'app-layout-tab',
  imports: [MatButtonToggleModule, MatCardModule, MatIconModule, EeLockComponent],
  styleUrl: './layout-tab.component.scss',
  templateUrl: './layout-tab.component.html',
})
export class LayoutTabComponent {
  protected readonly scope = inject(BuiltInPagesScope);

  protected readonly choices: LayoutChoice[] = [
    {
      name: 'centered',
      label: $localize`:@@Layout_centered:Centered`,
      what: $localize`:@@Layout_centered_what:The logo above, the card in the middle, the picture behind everything.`,
      sided: false,
    },
    {
      name: 'split',
      label: $localize`:@@Layout_split:Split`,
      what: $localize`:@@Layout_split_what:The picture takes a full-height half and carries the name; the form takes the other.`,
      sided: true,
    },
    {
      name: 'drawer',
      label: $localize`:@@Layout_drawer:Drawer`,
      what: $localize`:@@Layout_drawer_what:The picture keeps the whole frame; a panel against one edge carries logo and form.`,
      sided: true,
    },
    {
      name: 'banner',
      label: $localize`:@@Layout_banner:Banner`,
      what: $localize`:@@Layout_banner_what:A brand band across the top, the card under it. Holds up without a picture.`,
      sided: false,
    },
    {
      name: 'bare',
      label: $localize`:@@Layout_bare:Bare`,
      what: $localize`:@@Layout_bare_what:No card: the fields sit on the picture itself. For a photograph worth showing whole.`,
      sided: false,
    },
  ];

  protected readonly current = computed(() => this.scope.layout().name || 'centered');
  protected readonly side = computed(() => this.scope.layout().side || 'left');
  // The side means nothing to the three layouts that are not made of halves -
  // the control stays, disabled, rather than appearing and disappearing under
  // the cursor.
  protected readonly sided = computed(
    () => this.choices.find((c) => c.name === this.current())?.sided ?? false,
  );

  protected pick(c: LayoutChoice): void {
    this.scope.setLayout(c.sided ? { name: c.name, side: this.side() } : { name: c.name });
  }

  protected setSide(side: string): void {
    if (!this.sided()) return;
    this.scope.setLayout({ name: this.current(), side });
  }
}
