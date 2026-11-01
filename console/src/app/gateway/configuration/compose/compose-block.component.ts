import { NgTemplateOutlet } from '@angular/common';
import { Component, computed, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatTooltipModule } from '@angular/material/tooltip';
import { PortalPreviewComponent } from './portal-preview.component';
import { Element, Entity, Field, Recipe, elementKey, elementsOf, fieldsOf, listOrigin, same, short } from './compose.model';

// What a click on the loom says: the object whole from a side or out, a field
// from a side or cleared, an element in or out, a list in order, a pairing.
// What rides beside a route: its deposited spec, its files.
export type Media = { left?: { spec?: boolean; files?: number }; right?: { spec?: boolean; files?: number }; result?: { spec?: boolean; files?: number } };

export type PickEvent =
  | { type: 'whole'; path: string; from: 'left' | 'right' | 'none' }
  | { type: 'field'; path: string; from: 'left' | 'right' | 'unset' }
  | { type: 'element'; list: string; key: string; op: 'add' | 'drop' }
  | { type: 'order'; list: string; keys: string[] }
  | { type: 'pair'; kind: string; left: string; right: string }
  | { type: 'include'; path: string; on: boolean };

// One row of the loom: an object seen on both sides and in the result.
//
// No verbs. Every difference is a choice between two sides, made on a radio:
// the card or the value chosen is the one the result shows, and the frame is
// red until the operator said something. An object on both sides that
// differs has no object card - it would repeat what the field lines say -
// but a line per field that differs, on the three columns, and a list is its
// elements, each a chip in or out, dragged into order in the centre. The
// empty slot opposite an object one side has is a choice too: leave it out,
// or remove it.
@Component({
  selector: 'app-compose-block',
  imports: [NgTemplateOutlet, MatButtonModule, MatCheckboxModule, MatTooltipModule, PortalPreviewComponent],
  host: { '(keydown)': 'onKey($event)' },
  styles: [
    `
      :host {
        display: contents;
      }
      /* Off screen, a block is not laid out: a configuration of four hundred
         routes costs what the screen shows. */
      fieldset.block,
      .cell {
        content-visibility: auto;
        contain-intrinsic-size: auto 120px;
      }
      fieldset.block {
        grid-column: 1 / -1;
        margin: 0 -16px;
        padding: 6px 16px 12px;
        border: 1px solid var(--mat-sys-error);
        border-radius: 12px;
        min-width: 0;
        position: relative;
        z-index: 1;
      }
      fieldset.block.decided {
        border-color: var(--mat-sys-outline-variant);
      }
      fieldset.block > legend {
        padding: 0 8px;
        margin-left: 12px;
        font-size: 0.75rem;
        font-weight: 600;
        color: var(--mat-sys-error);
      }
      fieldset.block.decided > legend {
        color: var(--mat-sys-on-surface-variant);
        font-weight: 500;
      }
      .inner,
      .rowhead {
        display: grid;
        grid-template-columns: 1fr 1fr 1fr;
        column-gap: 56px;
        row-gap: 10px;
      }
      .rowhead {
        grid-column: 1 / -1;
        align-items: center;
        padding: 4px 12px 2px;
      }
      .rowhead .badges {
        grid-column: 1;
        display: flex;
        flex-wrap: wrap;
        gap: 6px;
      }
      .rowhead .incl {
        grid-column: 2;
        display: flex;
        justify-content: center;
      }
      .rowhead .warn {
        grid-column: 1 / -1;
      }
      .cell,
      .fpanel {
        min-width: 0;
        position: relative;
        z-index: 1;
      }
      .card {
        position: relative;
        border-radius: 12px;
        padding: 10px 12px 10px 16px;
        min-height: 56px;
        background: color-mix(in srgb, var(--mat-sys-surface-container-lowest) 55%, transparent);
        border: 1px solid var(--mat-sys-outline-variant);
        display: flex;
        flex-direction: column;
        gap: 4px;
        box-sizing: border-box;
      }
      .card[data-pick] {
        cursor: pointer;
      }
      .card[data-pick]:hover,
      .card[data-pick]:focus-visible,
      .fcell.val[data-fpick]:focus-visible,
      .el:focus-visible {
        border-color: var(--mat-sys-outline);
        outline: 2px solid var(--mat-sys-primary);
        outline-offset: 1px;
      }
      .card.inert {
        cursor: default;
      }
      .card::before {
        content: '';
        position: absolute;
        left: 0;
        top: 8px;
        bottom: 8px;
        width: 4px;
        border-radius: 4px;
        background: var(--mat-sys-outline-variant);
      }
      .card.side-left {
        background: color-mix(in srgb, var(--mk-left) 10%, transparent);
      }
      .card.side-right {
        background: color-mix(in srgb, var(--mk-right) 10%, transparent);
      }
      .card.side-left::before {
        background: var(--mk-left);
      }
      .card.side-right::before {
        background: var(--mk-right);
      }
      .card.side-left.taken {
        box-shadow: inset 0 0 0 1px var(--mk-left);
        background: color-mix(in srgb, var(--mk-left) 20%, transparent);
      }
      .card.side-right.taken {
        box-shadow: inset 0 0 0 1px var(--mk-right);
        background: color-mix(in srgb, var(--mk-right) 20%, transparent);
      }
      .card.absent {
        border-style: dashed;
        background: transparent;
        color: var(--mat-sys-on-surface-variant);
        flex-direction: row;
        align-items: center;
        gap: 8px;
        font-size: 0.75rem;
        flex-wrap: wrap;
      }
      .card.absent::before {
        display: none;
      }
      .card.absent.choice.on {
        border-style: solid;
        border-color: var(--mat-sys-outline);
        background: var(--mat-sys-surface-container-low);
      }
      .card.result {
        background: color-mix(in srgb, var(--mk-center) 12%, transparent);
        border-color: color-mix(in srgb, var(--mk-center) 30%, transparent);
      }
      .card.result::after {
        content: '';
        position: absolute;
        right: 0;
        top: 8px;
        bottom: 8px;
        width: 4px;
        border-radius: 4px;
        background: var(--mat-sys-outline-variant);
      }
      .card.result.from-left::before,
      .card.result.from-left::after {
        background: var(--mk-left);
      }
      .card.result.from-right::before,
      .card.result.from-right::after {
        background: var(--mk-right);
      }
      .card.result.from-both::before {
        background: var(--mk-left);
      }
      .card.result.from-both::after {
        background: var(--mk-right);
      }
      .card.result.warn {
        border-color: var(--mat-sys-error);
      }
      .card .top {
        display: flex;
        align-items: center;
        gap: 8px;
      }
      .card .name {
        font-weight: 500;
        flex: 1;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .card .id {
        font-family: var(--mk-mono, monospace);
        font-size: 0.7rem;
        color: var(--mat-sys-on-surface-variant);
      }
      .card .meta {
        font-size: 0.75rem;
        color: var(--mat-sys-on-surface-variant);
        display: flex;
        flex-wrap: wrap;
        gap: 4px 10px;
      }
      .badge {
        font-size: 0.7rem;
        font-weight: 600;
        padding: 2px 8px;
        border-radius: 6px;
        background: var(--mat-sys-surface-container-highest);
        color: var(--mat-sys-on-surface-variant);
        white-space: nowrap;
      }
      .badge.pulled {
        border: 1px dashed var(--mat-sys-outline);
        background: transparent;
      }
      .warn {
        font-size: 0.75rem;
        color: var(--mat-sys-error);
        line-height: 1.35;
      }
      .incl {
        display: inline-flex;
        align-items: center;
        margin-left: auto;
        white-space: nowrap;
      }
      .incl mat-checkbox {
        --mat-checkbox-selected-icon-color: var(--mk-center);
        --mat-checkbox-selected-focus-icon-color: var(--mk-center);
        --mat-checkbox-selected-hover-icon-color: var(--mk-center);
        font-size: 0.75rem;
      }
      /* Fields: one tinted panel per column, hung under its card or standing
         alone, the lines inside. */
      .fpanel {
        display: flex;
        flex-direction: column;
        gap: 4px;
        padding: 8px;
        border-radius: 12px;
        border: 1px solid transparent;
      }
      .fpanel.side-left {
        background: color-mix(in srgb, var(--mk-left) 12%, transparent);
        border-color: color-mix(in srgb, var(--mk-left) 40%, transparent);
      }
      .fpanel.side-right {
        background: color-mix(in srgb, var(--mk-right) 12%, transparent);
        border-color: color-mix(in srgb, var(--mk-right) 40%, transparent);
      }
      .fpanel.res {
        background: color-mix(in srgb, var(--mk-center) 12%, transparent);
        border-color: color-mix(in srgb, var(--mk-center) 40%, transparent);
      }
      .fcell {
        font-size: 0.75rem;
        min-height: 34px;
        display: flex;
        align-items: flex-start;
        padding: 4px 10px;
        border-radius: 8px;
        gap: 6px;
        background: color-mix(in srgb, var(--mat-sys-surface-container-lowest) 55%, transparent);
        border: 1px solid transparent;
        box-sizing: border-box;
        position: relative;
      }
      .fcell .k {
        font-weight: 500;
        font-size: 0.7rem;
        color: var(--mat-sys-on-surface-variant);
        min-width: 96px;
        margin-top: 3px;
      }
      .fcell .v {
        flex: 1;
        min-width: 0;
        font-family: var(--mk-mono, monospace);
        font-size: 0.72rem;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        margin-top: 3px;
      }
      .fcell .v.unset {
        font-style: italic;
        color: var(--mat-sys-on-surface-variant);
      }
      .fcell.val[data-fpick] {
        cursor: pointer;
      }
      .fcell.val[data-fpick]:hover {
        border-color: var(--mat-sys-outline);
      }
      .fcell.val.chosen.left {
        border-color: var(--mk-left);
        background: color-mix(in srgb, var(--mk-left) 18%, transparent);
      }
      .fcell.val.chosen.right {
        border-color: var(--mk-right);
        background: color-mix(in srgb, var(--mk-right) 18%, transparent);
      }
      .fcell.same {
        cursor: default;
      }
      .fcell.res.from-left {
        box-shadow:
          inset 3px 0 0 var(--mk-left),
          inset -3px 0 0 var(--mk-left);
      }
      .fcell.res.from-right {
        box-shadow:
          inset 3px 0 0 var(--mk-right),
          inset -3px 0 0 var(--mk-right);
      }
      .fcell.res.from-both {
        box-shadow:
          inset 3px 0 0 var(--mk-left),
          inset -3px 0 0 var(--mk-right);
      }
      /* The choice itself: a radio on each side, on where the result comes from. */
      .radio {
        width: 16px;
        height: 16px;
        border-radius: 50%;
        border: 2px solid var(--mat-sys-outline);
        flex-shrink: 0;
        margin-left: auto;
        position: relative;
        box-sizing: border-box;
        margin-top: 2px;
      }
      .radio.on::after {
        content: '';
        position: absolute;
        inset: 2px;
        border-radius: 50%;
      }
      .side-left .radio.on,
      .card.absent.choice .radio.on {
        border-color: var(--mk-left);
      }
      .side-left .radio.on::after,
      .card.absent.choice .radio.on::after {
        background: var(--mk-left);
      }
      .side-right .radio.on {
        border-color: var(--mk-right);
      }
      .side-right .radio.on::after {
        background: var(--mk-right);
      }
      .card.absent.choice .radio {
        margin-left: 0;
      }
      .fcell .radio {
        width: 14px;
        height: 14px;
        margin-left: 8px;
      }
      /* Chips: in the result = filled, the colour of its column; out = pale. */
      .chips {
        flex: 1;
        min-width: 0;
        display: flex;
        flex-wrap: wrap;
        gap: 4px;
      }
      .el {
        display: inline-block;
        max-width: 100%;
        padding: 1px 8px;
        border-radius: 10px;
        font-size: 0.7rem;
        border: 1px solid var(--mat-sys-outline-variant);
        cursor: pointer;
        font-family: var(--mk-mono, monospace);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        line-height: 18px;
        box-sizing: border-box;
      }
      .el.in {
        font-weight: 500;
        border-width: 1.5px;
      }
      .el.in.left,
      .side-left .el.in {
        border-color: var(--mk-left);
        background: color-mix(in srgb, var(--mk-left) 22%, var(--mat-sys-surface-container-lowest));
      }
      .el.in.right,
      .side-right .el.in {
        border-color: var(--mk-right);
        background: color-mix(in srgb, var(--mk-right) 22%, var(--mat-sys-surface-container-lowest));
      }
      .el.in.both {
        border-color: var(--mk-center);
        background: color-mix(in srgb, var(--mk-center) 22%, var(--mat-sys-surface-container-lowest));
      }
      .el.out {
        opacity: 0.5;
        border-style: dashed;
        background: transparent;
      }
      .el.out:hover {
        opacity: 0.8;
      }
      .el.ro {
        cursor: default;
      }
      .el[draggable='true'] {
        cursor: grab;
      }
      .el.dragover {
        outline: 2px solid var(--mk-center);
        outline-offset: 1px;
      }
      /* The x that unsets a value in the result, on hover of its line. The
         rules name the button: a value that is not set wears the class too. */
      button.unset {
        flex-shrink: 0;
        width: 18px;
        height: 18px;
        border-radius: 50%;
        border: 1px solid var(--mat-sys-outline);
        background: var(--mat-sys-surface-container-lowest);
        color: var(--mat-sys-on-surface-variant);
        font: 600 11px Roboto, sans-serif;
        line-height: 16px;
        padding: 0;
        cursor: pointer;
        opacity: 0;
        margin-left: 6px;
      }
      .fcell.res:hover button.unset {
        opacity: 1;
      }
      button.unset:hover {
        color: var(--mat-sys-error);
        border-color: var(--mat-sys-error);
      }
      .pairbtn {
        margin-left: auto;
      }
    `,
  ],
  template: `
    @let e = entity();
    @if (framed()) {
      <fieldset class="block" [class.decided]="decided()" [attr.data-row]="e.path">
        <legend>{{ e.label }}</legend>
        <div class="inner">
          <ng-container *ngTemplateOutlet="row" />
        </div>
      </fieldset>
    } @else {
      <ng-container *ngTemplateOutlet="row" />
    }

    <ng-template #row>
      @if (fieldsRow()) {
        <!-- An object on both sides that differs: its fields, on three columns. -->
        <div class="rowhead" [attr.data-head]="e.path">
          <span class="badges">
            @if (mediaLabel('result'); as m) {
              <span class="badge" matTooltip="Deposited beside the route, following it">{{ m }}</span>
            }
            @if (e.pairedWith) {
              <span class="badge" matTooltip="The result keeps {{ e.id }}; it was {{ e.pairedWith }} on the right">
                paired, id kept
              </span>
            }
          </span>
          <span class="incl">
            <mat-checkbox [checked]="!!e.result" (change)="pick.emit({ type: 'include', path: e.path, on: $event.checked })">
              in the result
            </mat-checkbox>
          </span>
          @for (w of warnings(); track w) {
            <div class="warn">{{ w }}</div>
          }
        </div>
        @if (e.result) {
          <ng-container *ngTemplateOutlet="panels" />
        } @else {
          <div class="cell"></div>
          <div class="cell"><div class="card absent"><span>not in the result</span></div></div>
          <div class="cell"></div>
        }
        @if (isPortal()) {
          <!-- The bar each side describes, a miniature in its column, under the fields. -->
          <div class="cell"><app-portal-preview [portal]="e.left" [name]="leftName()" /></div>
          <div class="cell"><app-portal-preview [portal]="e.result" name="Result" /></div>
          <div class="cell"><app-portal-preview [portal]="e.right" [name]="rightName()" /></div>
        }
      } @else {
        <!-- The object whole is the choice: a card on each side that has it,
             an empty slot opposite, the result between. -->
        <div class="cell">
          @if (e.left) {
            <div
              class="card side-left"
              [class.inert]="e.status === 'same'"
              [class.taken]="origin() === 'left'"
              [attr.data-pick]="e.status === 'same' ? null : e.path"
              [attr.tabindex]="e.status === 'same' ? null : 0"
              [attr.role]="e.status === 'same' ? null : 'radio'"
              [attr.data-card]="e.path + ':left'"
              (click)="e.status !== 'same' && pick.emit({ type: 'whole', path: e.path, from: 'left' })"
            >
              <div class="top">
                <span class="name">{{ e.label }}</span>
                @if (showId()) {
                  <span class="id">{{ e.id }}</span>
                }
                @if (e.status !== 'same') {
                  <span class="radio" [class.on]="origin() === 'left'"></span>
                }
              </div>
              <div class="meta"><span>{{ summary(e.left) }}</span>@if (mediaLabel('left'); as m) {<span class="badge">{{ m }}</span>}</div>
            </div>
          } @else {
            <div
              class="card absent choice"
              [class.on]="!e.result"
              [attr.data-pick]="e.path"
              tabindex="0"
              role="radio"
              matTooltip="Choose it: the object is left out of the result"
              (click)="pick.emit({ type: 'whole', path: e.path, from: 'none' })"
            >
              <span class="radio" [class.on]="!e.result"></span>
              <span>not in {{ leftName() }}</span>
            </div>
          }
        </div>
        <div class="cell">
          @if (e.result) {
            <div
              class="card result"
              [class.inert]="e.status === 'same'"
              [class.from-left]="e.status !== 'same' && origin() === 'left'"
              [class.from-right]="e.status !== 'same' && origin() === 'right'"
              [class.from-both]="e.status === 'same'"
              [class.warn]="warnings().length > 0"
            >
              <div class="top">
                <span class="name">{{ e.label }}</span>
                @if (e.status === 'same') {
                  <span class="incl">
                    <mat-checkbox checked (change)="pick.emit({ type: 'include', path: e.path, on: $event.checked })">
                      in the result
                    </mat-checkbox>
                  </span>
                }
              </div>
              @if (pulledBy().length || e.pairedWith || mediaLabel('result')) {
                <div class="meta">
                  @if (mediaLabel('result'); as m) {
                    <span class="badge" matTooltip="Deposited beside the route, following it">{{ m }}</span>
                  }
                  @if (pulledBy().length) {
                    <span class="badge pulled">added because {{ pulledBy().join(', ') }} names it</span>
                  }
                  @if (e.pairedWith) {
                    <span class="badge">paired, id kept: {{ e.id }}</span>
                  }
                </div>
              }
              @for (w of warnings(); track w) {
                <div class="warn">{{ w }}</div>
              }
            </div>
          } @else {
            <div class="card absent">
              @if (e.left && e.right) {
                <span>not in the result</span>
                <span class="incl">
                  <mat-checkbox (change)="pick.emit({ type: 'include', path: e.path, on: $event.checked })">
                    in the result
                  </mat-checkbox>
                </span>
              } @else if (e.left) {
                <span>not in the result: the gateway drops it</span>
              } @else {
                <span>not in the result</span>
              }
            </div>
          }
        </div>
        <div class="cell">
          @if (e.right) {
            <div
              class="card side-right"
              [class.inert]="e.status === 'same'"
              [class.taken]="origin() === 'right'"
              [attr.data-pick]="e.status === 'same' ? null : e.path"
              [attr.tabindex]="e.status === 'same' ? null : 0"
              [attr.role]="e.status === 'same' ? null : 'radio'"
              [attr.data-card]="e.path + ':right'"
              (click)="e.status !== 'same' && pick.emit({ type: 'whole', path: e.path, from: 'right' })"
            >
              <div class="top">
                <span class="name">{{ e.label }}</span>
                @if (showId()) {
                  <span class="id">{{ e.pairedWith ?? e.id }}</span>
                }
                @if (e.status !== 'same') {
                  <span class="radio" [class.on]="origin() === 'right'"></span>
                }
              </div>
              <div class="meta"><span>{{ summary(e.right) }}</span>@if (mediaLabel('right'); as m) {<span class="badge">{{ m }}</span>}</div>
            </div>
          } @else if (keepsRegardless()) {
            <div class="card absent"><span>not in {{ rightName() }}</span></div>
          } @else {
            <div
              class="card absent choice"
              [class.on]="!e.result"
              [attr.data-pick]="e.path"
              tabindex="0"
              role="radio"
              matTooltip="Choose it: the object is removed"
              (click)="pick.emit({ type: 'whole', path: e.path, from: 'none' })"
            >
              <span class="radio" [class.on]="!e.result"></span>
              <span>not in {{ rightName() }}</span>
              @if (pairProposal(); as other) {
                <button
                  matButton="outlined"
                  class="pairbtn"
                  matTooltip="Same name under another id: paired, the gateway updates {{ e.id }} instead of removing it and adding {{ other }}"
                  (click)="$event.stopPropagation(); pick.emit({ type: 'pair', kind: e.kind, left: e.id, right: other })"
                >
                  Pair with {{ other }}
                </button>
              }
            </div>
          }
        </div>
      }
    </ng-template>

    <!-- The field lines: one panel per column. -->
    <ng-template #panels>
      <div class="fpanel side-left" [attr.data-frow]="e.path">
        @for (f of shownFields(); track f.path) {
          @if (f.set) {
            <div
              class="fcell val side-left"
              [class.same]="f.same"
              [class.chosen]="f.left !== undefined && (f.same || listSide(f) === 'left')"
              [class.left]="f.left !== undefined && (f.same || listSide(f) === 'left')"
              [attr.data-fpick]="f.same || f.left === undefined ? null : f.path"
              [attr.tabindex]="f.same || f.left === undefined ? null : 0"
              [attr.role]="f.same || f.left === undefined ? null : \'radio\'"
              [attr.data-fcard]="fieldPath(f) + ':left'"
              (click)="!f.same && f.left !== undefined && pick.emit({ type: 'field', path: fieldPath(f), from: 'left' })"
            >
              <span class="k">{{ f.path }}</span>
              <span class="chips">
                @for (el of sideElements(f, 'left'); track el.key) {
                  <span
                    [class]="chipClass(el, f)"
                    [attr.tabindex]="f.same ? null : 0"
                    [attr.role]="f.same ? null : 'button'"
                    (click)="!f.same && toggle($event, f, el)"
                    >{{ chipLabel(el) }}</span
                  >
                }
              </span>
              @if (!f.same && f.left !== undefined) {
                <span class="radio" [class.on]="listSide(f) === 'left'"></span>
              }
            </div>
          } @else {
            <div
              class="fcell val side-left"
              [class.same]="f.same"
              [class.chosen]="f.left !== undefined && (f.same || scalarSide(f) === 'left')"
              [class.left]="f.left !== undefined && (f.same || scalarSide(f) === 'left')"
              [attr.data-fpick]="f.same || f.left === undefined ? null : f.path"
              [attr.tabindex]="f.same || f.left === undefined ? null : 0"
              [attr.role]="f.same || f.left === undefined ? null : \'radio\'"
              [attr.data-fcard]="fieldPath(f) + ':left'"
              (click)="!f.same && f.left !== undefined && pick.emit({ type: 'field', path: fieldPath(f), from: 'left' })"
            >
              <span class="k">{{ f.path }}</span>
              <span class="v" [class.unset]="f.left === undefined">{{ f.left === undefined || f.left === '' ? 'not set' : short(f.left) }}</span>
              @if (!f.same && f.left !== undefined) {
                <span class="radio" [class.on]="scalarSide(f) === 'left'"></span>
              }
            </div>
          }
        }
      </div>
      <div class="fpanel res">
        @for (f of shownFields(); track f.path) {
          @if (f.set) {
            <div class="fcell res" [class]="'fcell res ' + listClass(f)" [attr.data-fcard]="fieldPath(f) + ':res'">
              <span class="k">{{ f.path }}</span>
              <span class="chips">
                @for (el of elements(f); track el.key) {
                  <span
                    [class]="chipClass(el, f)"
                    [attr.tabindex]="f.same ? null : 0"
                    [attr.role]="f.same ? null : 'button'"
                    [attr.draggable]="f.same ? null : 'true'"
                    [attr.data-key]="el.key"
                    (click)="!f.same && toggle($event, f, el)"
                    (dragstart)="dragStart(f, el)"
                    (dragover)="dragOver($event, f)"
                    (dragleave)="dragLeave($event)"
                    (drop)="drop($event, f, el)"
                    >{{ chipLabel(el) }}</span
                  >
                }
              </span>
              @if (!f.same && hasElements(f)) {
                <button class="unset" matTooltip="Not set in the result" (click)="pick.emit({ type: 'field', path: fieldPath(f), from: 'unset' })">x</button>
              }
            </div>
          } @else {
            <div class="fcell res" [class]="'fcell res ' + (f.same ? 'same from-both' : 'from-' + scalarSide(f))" [attr.data-fcard]="fieldPath(f) + ':res'">
              <span class="k">{{ f.path }}</span>
              <span class="v" [class.unset]="f.result === undefined">{{ f.result === undefined || f.result === '' ? 'not set' : short(f.result) }}</span>
              @if (!f.same && f.result !== undefined) {
                <button class="unset" matTooltip="Not set in the result" (click)="pick.emit({ type: 'field', path: fieldPath(f), from: 'unset' })">x</button>
              }
            </div>
          }
        }
      </div>
      <div class="fpanel side-right">
        @for (f of shownFields(); track f.path) {
          @if (f.set) {
            <div
              class="fcell val side-right"
              [class.same]="f.same"
              [class.chosen]="f.right !== undefined && (f.same || listSide(f) === 'right')"
              [class.right]="f.right !== undefined && (f.same || listSide(f) === 'right')"
              [attr.data-fpick]="f.same || f.right === undefined ? null : f.path"
              [attr.tabindex]="f.same || f.right === undefined ? null : 0"
              [attr.role]="f.same || f.right === undefined ? null : \'radio\'"
              [attr.data-fcard]="fieldPath(f) + ':right'"
              (click)="!f.same && f.right !== undefined && pick.emit({ type: 'field', path: fieldPath(f), from: 'right' })"
            >
              <span class="k">{{ f.path }}</span>
              <span class="chips">
                @for (el of sideElements(f, 'right'); track el.key) {
                  <span
                    [class]="chipClass(el, f)"
                    [attr.tabindex]="f.same ? null : 0"
                    [attr.role]="f.same ? null : 'button'"
                    (click)="!f.same && toggle($event, f, el)"
                    >{{ chipLabel(el) }}</span
                  >
                }
              </span>
              @if (!f.same && f.right !== undefined) {
                <span class="radio" [class.on]="listSide(f) === 'right'"></span>
              }
            </div>
          } @else {
            <div
              class="fcell val side-right"
              [class.same]="f.same"
              [class.chosen]="f.right !== undefined && (f.same || scalarSide(f) === 'right')"
              [class.right]="f.right !== undefined && (f.same || scalarSide(f) === 'right')"
              [attr.data-fpick]="f.same || f.right === undefined ? null : f.path"
              [attr.tabindex]="f.same || f.right === undefined ? null : 0"
              [attr.role]="f.same || f.right === undefined ? null : \'radio\'"
              [attr.data-fcard]="fieldPath(f) + ':right'"
              (click)="!f.same && f.right !== undefined && pick.emit({ type: 'field', path: fieldPath(f), from: 'right' })"
            >
              <span class="k">{{ f.path }}</span>
              <span class="v" [class.unset]="f.right === undefined">{{ f.right === undefined || f.right === '' ? 'not set' : short(f.right) }}</span>
              @if (!f.same && f.right !== undefined) {
                <span class="radio" [class.on]="scalarSide(f) === 'right'"></span>
              }
            </div>
          }
        }
      </div>
    </ng-template>
  `,
})
export class ComposeBlockComponent {
  readonly entity = input.required<Entity>();
  readonly recipe = input.required<Recipe>();
  // Bumped by the page at each change of the recipe, so the computed
  // values below read it again (the recipe is a plain object).
  readonly version = input(0);
  readonly leftName = input('');
  readonly rightName = input('');
  readonly onlyDiff = input(true);
  // What the report says about this object: who pulled it, what it names
  // that nothing declares.
  readonly pulledBy = input<string[]>([]);
  readonly warnings = input<string[]>([]);
  // The right id proposed as the same object under another id.
  readonly pairProposal = input<string | undefined>(undefined);
  // The spec and files a route carries, per side: they ride beside the
  // document and follow the route in silence otherwise.
  readonly media = input<Media | null>(null);
  // Route names from both documents: what names a portal entry that has
  // no label of its own.
  readonly routeNames = input<ReadonlyMap<string, string>>(new Map());
  readonly pick = output<PickEvent>();

  protected mediaLabel(side: 'left' | 'right' | 'result'): string {
    const m = this.media()?.[side];
    if (!m) return '';
    const parts = [];
    if (m.spec) parts.push('spec');
    if (m.files) parts.push(m.files === 1 ? '1 file' : `${m.files} files`);
    return parts.join(', ');
  }

  // Enter or space on a focused card, value or chip is the click.
  protected onKey(ev: KeyboardEvent): void {
    if (ev.key !== 'Enter' && ev.key !== ' ') return;
    const el = (ev.target as HTMLElement).closest<HTMLElement>('[data-pick], [data-fpick], .el[tabindex], button.unset, .pairbtn');
    if (!el) return;
    ev.preventDefault();
    el.click();
  }

  protected readonly short = short;

  // Framed in red until decided: everything the sides disagree on, except
  // what the composition added by itself.
  protected readonly framed = computed(() => {
    const e = this.entity();
    if (this.pulledBy().length) return false;
    return e.status !== 'same';
  });
  protected readonly decided = computed(() => {
    this.version();
    return this.recipe().touched(this.entity().path) || !!this.entity().pairedWith;
  });
  // The portal setting is drawn, not only listed: the bar on each side.
  protected readonly isPortal = computed(() => this.entity().kind === 'setting' && this.entity().id === 'portal');
  // An object on both sides that differs is its fields, not a card.
  protected readonly fieldsRow = computed(() => this.entity().status === 'update' && !this.pulledBy().length);
  protected readonly origin = computed(() => this.entity().origin);
  protected readonly fields = computed(() => {
    this.version();
    return fieldsOf(this.entity());
  });
  protected readonly shownFields = computed(() => (this.onlyDiff() ? this.fields().filter((f) => !f.same) : this.fields()));
  // A generated id says nothing; it is shown only where it is the point.
  protected readonly showId = computed(() => !!this.pairProposal() || !!this.entity().pairedWith);
  // A tenant is never removed by a configuration: no choice opposite it.
  protected readonly keepsRegardless = computed(() => this.entity().kind === 'tenant');

  protected summary(o: Record<string, unknown>): string {
    const e = this.entity();
    switch (e.kind) {
      case 'route':
        return `${o['upstream'] ?? ''} ${o['enabled'] === false ? '(disabled)' : ''}`.trim();
      case 'role':
        return o['parent'] ? `parent ${o['parent']}` : 'top-level';
      case 'group':
        return `${o['tenantId'] ?? ''}: ${(Array.isArray(o['roles']) ? o['roles'] : []).join(', ')}`;
      case 'oidcClient':
        return o['confidential'] ? 'confidential' : 'public';
      default:
        return typeof o['description'] === 'string' ? (o['description'] as string) : short(o).slice(0, 80);
    }
  }

  protected fieldPath(f: Field): string {
    return `${this.entity().path}/${f.path}`;
  }

  // Where a scalar's result comes from: a side whose value it is.
  protected scalarSide(f: Field): 'left' | 'right' | 'none' {
    if (f.result === undefined) return 'none';
    if (f.left !== undefined && same(f.result, f.left)) return 'left';
    if (f.right !== undefined && same(f.result, f.right)) return 'right';
    return 'none';
  }

  // The side whose list the result IS, as is.
  protected listSide(f: Field): 'left' | 'right' | 'none' {
    if (f.left !== undefined && same(f.result ?? [], f.left)) return 'left';
    if (f.right !== undefined && same(f.result ?? [], f.right)) return 'right';
    return 'none';
  }

  protected elements(f: Field): Element[] {
    return elementsOf(f);
  }

  protected sideElements(f: Field, side: 'left' | 'right'): Element[] {
    const byKey = new Map(elementsOf(f).map((el) => [el.key, el]));
    const list = side === 'left' ? f.left : f.right;
    return (Array.isArray(list) ? list : []).map((v) => byKey.get(elementKey(v))!).filter(Boolean);
  }

  protected hasElements(f: Field): boolean {
    return Array.isArray(f.result) && f.result.length > 0;
  }

  protected listClass(f: Field): string {
    if (f.same) return 'same from-both';
    const o = listOrigin(elementsOf(f));
    return o === 'empty' ? '' : o === 'common' ? 'from-left' : 'from-' + o;
  }

  protected chipClass(el: Element, f: Field): string {
    if (f.same) return 'el in both ro';
    const where = el.inL && el.inR ? 'both' : el.inL ? 'left' : 'right';
    return `el ${el.inRes ? 'in' : 'out'} ${where}`;
  }

  // What names an element that is an object: its label, name, title,
  // description or id; a brick by its type and first argument; else its JSON.
  protected chipLabel(el: Element): string {
    const v = el.val;
    if (!v || typeof v !== 'object' || Array.isArray(v)) return short(v);
    const o = v as Record<string, unknown>;
    for (const k of ['label', 'name', 'title', 'description', 'id']) {
      if (typeof o[k] === 'string' && o[k]) return o[k] as string;
    }
    if (typeof o['routeId'] === 'string') return this.routeNames().get(o['routeId']) ?? o['routeId'];
    if (o['special'] === 'notifications') return 'Notifications';
    if (typeof o['type'] === 'string') {
      const args = o['args'];
      const first = args && typeof args === 'object' ? Object.values(args as Record<string, unknown>)[0] : undefined;
      return first === undefined ? (o['type'] as string) : `${o['type']} ${short(first)}`;
    }
    if (typeof o['from'] === 'string' && typeof o['to'] === 'string') return `${o['from']} -> ${o['to']}`;
    for (const k of ['field', 'header', 'claim', 'key', 'pattern', 'path']) {
      if (typeof o[k] === 'string' && o[k]) return o[k] as string;
    }
    return short(v);
  }

  protected toggle(ev: Event, f: Field, el: Element): void {
    ev.stopPropagation();
    this.pick.emit({ type: 'element', list: this.fieldPath(f), key: el.key, op: el.inRes ? 'drop' : 'add' });
  }

  // Ordering: a chip dragged onto another of the same list lands before it.
  private readonly dragging = signal<{ list: string; key: string } | null>(null);

  protected dragStart(f: Field, el: Element): void {
    this.dragging.set({ list: this.fieldPath(f), key: el.key });
  }

  protected dragOver(ev: DragEvent, f: Field): void {
    const d = this.dragging();
    if (!d || d.list !== this.fieldPath(f)) return;
    ev.preventDefault();
    (ev.currentTarget as HTMLElement).classList.add('dragover');
  }

  protected dragLeave(ev: DragEvent): void {
    (ev.currentTarget as HTMLElement).classList.remove('dragover');
  }

  protected drop(ev: DragEvent, f: Field, target: Element): void {
    const d = this.dragging();
    (ev.currentTarget as HTMLElement).classList.remove('dragover');
    if (!d || d.list !== this.fieldPath(f)) return;
    ev.preventDefault();
    const keys = elementsOf(f)
      .map((el) => el.key)
      .filter((k) => k !== d.key);
    keys.splice(keys.indexOf(target.key), 0, d.key);
    this.dragging.set(null);
    this.pick.emit({ type: 'order', list: this.fieldPath(f), keys });
  }
}
