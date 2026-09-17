import { Component, ElementRef, effect, inject, input } from '@angular/core';

// Renders an icon stored as an SVG string (from the icon bank) as a CSS mask,
// so it takes the current text colour and needs no icon font. The mask is set
// straight on the host element (not through a style binding) to sidestep
// Angular's style sanitiser stripping the data: URI.
@Component({
  selector: 'app-svg-icon',
  template: '',
  styles: `
    :host {
      display: inline-block;
      width: var(--icon-size, 20px);
      height: var(--icon-size, 20px);
      background: currentColor;
      -webkit-mask-size: contain;
      mask-size: contain;
      -webkit-mask-repeat: no-repeat;
      mask-repeat: no-repeat;
      -webkit-mask-position: center;
      mask-position: center;
      flex: 0 0 auto;
    }
  `,
})
export class SvgIconComponent {
  readonly svg = input('');
  private readonly el = inject(ElementRef<HTMLElement>);

  constructor() {
    effect(() => {
      const s = this.svg();
      const uri = s ? `url("data:image/svg+xml;utf8,${encodeURIComponent(s)}")` : '';
      const style = (this.el.nativeElement as HTMLElement).style;
      style.setProperty('-webkit-mask-image', uri);
      style.setProperty('mask-image', uri);
    });
  }
}
