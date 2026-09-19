import { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    pathMatch: 'full',
    loadComponent: () => import('./pages/about.component').then((m) => m.AboutComponent),
  },
  {
    path: 'requirements',
    loadComponent: () =>
      import('./pages/requirements.component').then((m) => m.RequirementsComponent),
  },
  {
    path: 'dev-mode',
    loadComponent: () => import('./pages/dev-mode.component').then((m) => m.DevModeComponent),
  },
  {
    path: 'deploy',
    loadComponent: () => import('./pages/deploy.component').then((m) => m.DeployComponent),
  },
  {
    path: 'cluster',
    loadComponent: () => import('./pages/cluster.component').then((m) => m.ClusterComponent),
  },
  {
    path: 'roadmap',
    loadComponent: () => import('./pages/roadmap.component').then((m) => m.RoadmapComponent),
  },
  {
    path: 'performance',
    loadComponent: () =>
      import('./pages/performance.component').then((m) => m.PerformanceComponent),
  },
  {
    path: 'tests',
    loadComponent: () => import('./pages/tests.component').then((m) => m.TestsComponent),
  },
  // The documentation proper. One shell, and every page under it is the same
  // component reading the JSON the build produced - so a page added under
  // docs/content needs no route of its own.
  {
    path: 'docs',
    loadComponent: () => import('./docs/docs-shell.component').then((m) => m.DocsShellComponent),
    children: [
      {
        path: '**',
        loadComponent: () => import('./docs/docs-page.component').then((m) => m.DocsPageComponent),
      },
    ],
  },
  { path: '**', redirectTo: '' },
];
