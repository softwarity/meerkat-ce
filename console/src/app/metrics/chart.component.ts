import {
  Component,
  ElementRef,
  OnDestroy,
  afterNextRender,
  effect,
  inject,
  input,
  viewChild,
} from '@angular/core';
import * as echarts from 'echarts/core';
import { LineChart } from 'echarts/charts';
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';

// Only what is drawn is linked in. The full bundle is about a megabyte; the
// four pieces below are what a line chart needs, and the console is served by
// the gateway itself.
echarts.use([LineChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer]);

// One chart, told what to draw.
//
// A component and not a directive, so the element it needs is its own and
// nobody has to remember to give it a height. It knows nothing about traffic:
// what it takes is an ECharts option, which is what makes the second chart on
// this screen - and the one on the audit screen after it - free.
@Component({
  selector: 'app-chart',
  template: '<div #host class="host"></div>',
  styles: [
    `
      :host {
        display: block;
      }
      .host {
        width: 100%;
        height: 100%;
      }
    `,
  ],
})
export class ChartComponent implements OnDestroy {
  readonly option = input.required<echarts.EChartsCoreOption>();
  private readonly host = viewChild.required<ElementRef<HTMLDivElement>>('host');
  private chart?: echarts.ECharts;
  private observer?: ResizeObserver;

  constructor() {
    const element = inject(ElementRef).nativeElement as HTMLElement;
    afterNextRender(() => {
      this.chart = echarts.init(this.host().nativeElement, undefined, { renderer: 'canvas' });
      // ECharts sizes itself once, from the element it was given. A drawer
      // opening, a window resized or a rail collapsing all change that element
      // without telling it, and a chart that keeps its first size is the shape
      // this ends up as.
      this.observer = new ResizeObserver(() => this.chart?.resize());
      this.observer.observe(element);
      this.chart.setOption(this.option());
    });
    effect(() => {
      const option = this.option();
      // notMerge: false keeps the axes and the zoom while the data moves,
      // which is what a live chart is - the alternative redraws from nothing
      // five times a minute and loses whatever the reader was pointing at.
      this.chart?.setOption(option);
    });
  }

  ngOnDestroy(): void {
    this.observer?.disconnect();
    this.chart?.dispose();
  }
}
